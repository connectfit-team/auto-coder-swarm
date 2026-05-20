package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/connectfit-team/auto-coder-swarm/internal/gitmgr"
	"github.com/connectfit-team/auto-coder-swarm/internal/insightclient"
	"github.com/connectfit-team/auto-coder-swarm/internal/llm"
	"github.com/connectfit-team/auto-coder-swarm/internal/orchestrator"
	"github.com/connectfit-team/auto-coder-swarm/internal/storage"
	"github.com/connectfit-team/auto-coder-swarm/internal/workspace"
)

const webhookURL = "https://hooks.slack.com/services/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"

func sendToSlack(message string) {
	payload := map[string]string{"text": message}
	b, _ := json.Marshal(payload)
	http.Post(webhookURL, "application/json", bytes.NewBuffer(b))
}

func worker(id int, orc *orchestrator.SwarmOrchestrator, store *storage.Storage) {
	for {
		task, err := store.GetNextPendingTask()
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}

		log.Printf("[Worker %d] Processing Task #%d: %s", id, task.ID, task.UserRequest)
		sendToSlack(fmt.Sprintf("🤖 *Task #%d 시작*: %s", task.ID, task.UserRequest))
		store.UpdateTaskStatus(task.ID, storage.StatusRunning, "", "")

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		
		var lockedRepo string
		lockFunc := func(repoName string) (bool, error) {
			ok, err := store.TryLockRepo(repoName, task.ID)
			if ok { 
				lockedRepo = repoName 
				store.UpdateTaskRepo(task.ID, repoName)
			}
			return ok, err
		}

		res, err := orc.RunTask(ctx, task.UserRequest, lockFunc)
		cancel()

		if lockedRepo != "" { store.UnlockRepo(lockedRepo) }

		if err != nil {
			log.Printf("❌ Task #%d failed: %v", task.ID, err)
			sendToSlack(fmt.Sprintf("❌ *Task #%d 실패*: %v", task.ID, err))
			store.UpdateTaskStatus(task.ID, storage.StatusFailed, "", err.Error())
		} else {
			log.Printf("✅ Task #%d completed: %s", task.ID, res.PRURL)
			sendToSlack(fmt.Sprintf("✅ *Task #%d 성공!*\n🔗 *PR*: %s", task.ID, res.PRURL))
			store.UpdateTaskStatus(task.ID, storage.StatusCompleted, res.PRURL, "")
		}
	}
}

func main() {
	log.Println("🚀 Auto-Coder Swarm Starting (Phase 6: Real PR Reporting Ready)")

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

	http.HandleFunc("/request", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		if query == "" { return }
		task, _ := store.CreateTask(query)
		fmt.Fprintf(w, "Task #%d queued", task.ID)
	})

	log.Fatal(http.ListenAndServe(":8006", nil))
}
