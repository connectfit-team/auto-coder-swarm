package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/connectfit-team/auto-coder-swarm/internal/api"
	"github.com/connectfit-team/auto-coder-swarm/internal/gitmgr"
	"github.com/connectfit-team/auto-coder-swarm/internal/insightclient"
	"github.com/connectfit-team/auto-coder-swarm/internal/llm"
	"github.com/connectfit-team/auto-coder-swarm/internal/orchestrator"
	"github.com/connectfit-team/auto-coder-swarm/internal/storage"
	"github.com/connectfit-team/auto-coder-swarm/internal/voter"
	"github.com/connectfit-team/auto-coder-swarm/internal/web"
	"github.com/connectfit-team/auto-coder-swarm/internal/worker"
	"github.com/connectfit-team/auto-coder-swarm/internal/workspace"
	"github.com/connectfit-team/auto-coder-swarm/internal/stream"
	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
)

const webhookURL = "https://hooks.slack.com/services/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"

func sendToSlack(message string) {
	payload := map[string]string{"text": message}
	b, _ := json.Marshal(payload)
	http.Post(webhookURL, "application/json", bytes.NewBuffer(b))
}

// StreamAdapter connects the stream manager to the agent layer
type StreamAdapter struct {
	manager *stream.Manager
}

func (a *StreamAdapter) Broadcast(taskID uint, agentName, message string) {
	a.manager.Broadcast(stream.Thought{
		TaskID:    taskID,
		AgentName: agentName,
		Message:   message,
	})
}

func taskWorker(id int, orc *orchestrator.SwarmOrchestrator, store *storage.Storage, wm *worker.Manager) {
	for {
		task, err := store.ClaimNextTask()
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}

		log.Printf("[Worker %d] Task #%d (Status: %s)", id, task.ID, task.Status)

		// Inject task_id into context for streaming
		ctx := context.WithValue(context.Background(), "task_id", task.ID)
		ctx, cancel := context.WithCancel(ctx)
		wm.Register(task.ID, cancel)

		lockFunc := func(repoName string) (bool, error) {
			ok, err := store.TryLockRepo(repoName, task.ID)
			if ok {
				store.UpdateTaskRepo(task.ID, repoName)
			}
			return ok, err
		}

		var statelessReq orchestrator.StatelessRequest
		if err := json.Unmarshal([]byte(task.UserRequest), &statelessReq); err != nil {
			statelessReq = orchestrator.StatelessRequest{UserRequest: task.UserRequest}
		}

		isApproved := task.Status == storage.StatusApproved
		res, err := orc.RunStatelessTask(ctx, task.ID, statelessReq, isApproved, lockFunc)
		
		wm.Unregister(task.ID)
		cancel()

		if res.RepoName != "" {
			store.UnlockRepo(res.RepoName)
		}

		if err != nil {
			if ctx.Err() == context.Canceled {
				log.Printf("🚫 Task #%d was stopped by user.", task.ID)
			} else {
				sendToSlack(fmt.Sprintf("❌ *Task #%d 실패*: %v", task.ID, err))
				store.UpdateTaskStatus(task.ID, storage.StatusFailed, "", err.Error())
			}
		} else if res.WaitingApproval {
			store.UpdateTaskStatus(task.ID, storage.StatusWaitingApproval, "", "")
			sendToSlack(fmt.Sprintf("⏳ *Task #%d 검증 완료*: 승인이 필요합니다.\n🔗 `POST http://192.168.120.54:8006/api/v1/approve?id=%d`", task.ID, task.ID))
		} else {
			sendToSlack(fmt.Sprintf("✅ *Task #%d 성공!*\n📍 *Repo*: %s\n🔗 *PR*: %s", task.ID, res.RepoName, res.PRURL))
			store.UpdateTaskStatus(task.ID, storage.StatusCompleted, res.PRURL, "")

			for _, chainReq := range res.ChainTasks {
				b, _ := json.Marshal(chainReq)
				newToken, _ := store.CreateTask(string(b))
				sendToSlack(fmt.Sprintf("🔗 *연쇄 작업 발견!* (#%d): %s 레포지토리 수정 예약됨.", newToken.ID, chainReq.TargetRepo))
			}
		}
	}
}

func main() {
	log.Println("🚀 Auto-Coder Swarm Starting (Phase 9: Enterprise Readiness)")

	dbPath := "/home/cnf/projects/auto-coder-swarm/swarm.db"
	store, err := storage.NewStorage(dbPath)
	if err != nil {
		log.Fatalf("❌ DB init failed: %v", err)
	}
	store.ResetRunningToPending()

	wm := worker.NewManager()
	sm := stream.NewManager(store)
	
	// Plug the stream manager and storage into the global agent layer
	agent.GlobalStream = &StreamAdapter{manager: sm}
	agent.GlobalStorage = store
	
	baseURL := "http://localhost:11434"
	gemma4 := llm.NewOllamaModel("gemma4:31b", baseURL)
	llama3 := llm.NewOllamaModel("llama3:70b-instruct-q8_0", baseURL)
	qwen := llm.NewOllamaModel("qwen2.5vl:32b", baseURL)
	
	v := voter.NewMultiModelVoter(gemma4, llama3, qwen)

	ic := insightclient.NewClient("http://localhost:8005")
	wsMgr := workspace.NewLocalManager("/tmp", "/home/cnf/projects/code-insight-engine/repos")
	gitSvc := gitmgr.NewGitManager()
	orc := orchestrator.NewSwarmOrchestrator(ic, wsMgr, gitSvc, gemma4, store, v)

	workerCount := 3
	log.Printf("⚙️ Starting %d concurrent swarm workers...", workerCount)
	for w := 1; w <= workerCount; w++ {
		go taskWorker(w, orc, store, wm)
	}

	mux := http.NewServeMux()
	handler := api.NewSwarmHandler(store)
	handler.RegisterRoutes(mux)
	dashHandler := web.NewDashboardHandler(store, wm, sm, "/home/cnf/projects/auto-coder-swarm/web/templates")
	dashHandler.RegisterRoutes(mux)

	log.Println("📡 Swarm API & Dashboard & CoT Stream listening on :8006")
	log.Fatal(http.ListenAndServe(":8006", mux))
}
