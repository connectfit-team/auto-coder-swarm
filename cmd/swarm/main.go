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
	"bytes"
	"fmt"
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

		log.Printf("[Worker %d] Processing Task #%d (Status: %s)", id, task.ID, task.Status)
		store.UpdateTaskStatus(task.ID, storage.StatusRunning, "", "")

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		
		lockFunc := func(repoName string) (bool, error) {
			ok, err := store.TryLockRepo(repoName, task.ID)
			if ok { store.UpdateTaskRepo(task.ID, repoName) }
			return ok, err
		}

		var statelessReq orchestrator.StatelessRequest
		json.Unmarshal([]byte(task.UserRequest), &statelessReq)
		
		// If status is APPROVED, it means it's a resume from human check
		isApproved := task.Status == storage.StatusApproved

		res, err := orc.RunStatelessTask(ctx, statelessReq, isApproved, lockFunc)
		cancel()

		if res.RepoName != "" { store.UnlockRepo(res.RepoName) }

		if err != nil {
			sendToSlack(fmt.Sprintf("❌ *Task #%d 실패*: %v", task.ID, err))
			store.UpdateTaskStatus(task.ID, storage.StatusFailed, "", err.Error())
		} else if res.WaitingApproval {
			store.UpdateTaskStatus(task.ID, storage.StatusWaitingApproval, "", "")
			sendToSlack(fmt.Sprintf("⏳ *Task #%d 검증 완료*: 수정을 확인하고 승인해주세요.\n🔗 *승인 API*: `POST http://192.168.120.54:8006/api/v1/approve?id=%d`", task.ID, task.ID))
		} else {
			sendToSlack(fmt.Sprintf("✅ *Task #%d 성공!*\n🔗 *PR*: %s", task.ID, res.PRURL))
			store.UpdateTaskStatus(task.ID, storage.StatusCompleted, res.PRURL, "")
		}
	}
}

func main() {
	log.Println("🚀 Auto-Coder Swarm Starting (Phase 7: Human-in-the-Loop Ready)")

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
