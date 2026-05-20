package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/connectfit-team/auto-coder-swarm/internal/api"
	"github.com/connectfit-team/auto-coder-swarm/internal/gitmgr"
	"github.com/connectfit-team/auto-coder-swarm/internal/insightclient"
	"github.com/connectfit-team/auto-coder-swarm/internal/llm"
	"github.com/connectfit-team/auto-coder-swarm/internal/orchestrator"
	"github.com/connectfit-team/auto-coder-swarm/internal/storage"
	"github.com/connectfit-team/auto-coder-swarm/internal/workspace"
	"encoding/json"
)

func worker(id int, orc *orchestrator.SwarmOrchestrator, store *storage.Storage) {
	for {
		task, err := store.GetNextPendingTask()
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}

		log.Printf("[Worker %d] Processing Task #%d", id, task.ID)
		store.UpdateTaskStatus(task.ID, storage.StatusRunning, "", "")

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		
		lockFunc := func(repoName string) (bool, error) {
			ok, err := store.TryLockRepo(repoName, task.ID)
			if ok { store.UpdateTaskRepo(task.ID, repoName) }
			return ok, err
		}

		var statelessReq orchestrator.StatelessRequest
		if err := json.Unmarshal([]byte(task.UserRequest), &statelessReq); err != nil {
			statelessReq = orchestrator.StatelessRequest{UserRequest: task.UserRequest}
		}
		
		res, err := orc.RunStatelessTask(ctx, statelessReq, lockFunc)
		cancel()

		if res.RepoName != "" { store.UnlockRepo(res.RepoName) }

		if err != nil {
			log.Printf("❌ Task #%d failed: %v", task.ID, err)
			store.UpdateTaskStatus(task.ID, storage.StatusFailed, "", err.Error())
		} else {
			log.Printf("✅ Task #%d completed: %s", task.ID, res.PRURL)
			store.UpdateTaskStatus(task.ID, storage.StatusCompleted, res.PRURL, "")
		}
	}
}

func main() {
	log.Println("🚀 Auto-Coder Swarm Starting (Phase 6: Formal API Gateway Ready)")

	dbPath := "/home/cnf/projects/auto-coder-swarm/swarm.db"
	store, err := storage.NewStorage(dbPath)
	if err != nil { log.Fatalf("❌ DB init failed: %v", err) }
	store.ResetRunningToPending()

	ollamaModel := llm.NewOllamaModel("gemma4:31b", "http://localhost:11434")
	ic := insightclient.NewClient("http://localhost:8005")
	wsMgr := workspace.NewLocalManager("/tmp", "/home/cnf/projects/code-insight-engine/repos")
	gitSvc := gitmgr.NewGitManager()
	orc := orchestrator.NewSwarmOrchestrator(ic, wsMgr, gitSvc, ollamaModel)

	for w := 1; w <= 1; w++ { go worker(w, orc, store) }

	mux := http.NewServeMux()
	handler := api.NewSwarmHandler(store)
	handler.RegisterRoutes(mux)

	log.Println("📡 API Gateway listening on :8006")
	log.Fatal(http.ListenAndServe(":8006", mux))
}
