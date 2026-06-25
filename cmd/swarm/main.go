package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
	"github.com/connectfit-team/auto-coder-swarm/internal/api"
	"github.com/connectfit-team/auto-coder-swarm/internal/bus"
	"github.com/connectfit-team/auto-coder-swarm/internal/ckhclient"
	"github.com/connectfit-team/auto-coder-swarm/internal/gitmgr"
	"github.com/connectfit-team/auto-coder-swarm/internal/insightclient"
	"github.com/connectfit-team/auto-coder-swarm/internal/llm"
	"github.com/connectfit-team/auto-coder-swarm/internal/orchestrator"
	"github.com/connectfit-team/auto-coder-swarm/internal/reporting"
	"github.com/connectfit-team/auto-coder-swarm/internal/security"
	"github.com/connectfit-team/auto-coder-swarm/internal/storage"
	"github.com/connectfit-team/auto-coder-swarm/internal/stream"
	"github.com/connectfit-team/auto-coder-swarm/internal/web"
	"github.com/connectfit-team/auto-coder-swarm/internal/worker"
	"github.com/connectfit-team/auto-coder-swarm/internal/workspace"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
)

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok { return value }
	return fallback
}

type StreamAdapter struct {
	manager *stream.Manager
}

func (a *StreamAdapter) Broadcast(taskID string, agentName, message string) {
	a.manager.Broadcast(stream.Thought{
		TaskID:    taskID,
		AgentName: agentName,
		Message:   message,
	})
}

func sendToSlack(webhookURL, message string) {
	if webhookURL == "" { return }
	payload := map[string]string{"text": message}
	b, _ := json.Marshal(payload)
	http.Post(webhookURL, "application/json", bytes.NewBuffer(b))
}

func taskWorker(id int, orc *orchestrator.SwarmOrchestrator, store *storage.Storage, wm *worker.Manager, slackWebhook string) {
	for {
		task, err := store.ClaimNextTask()
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}

		log.Printf("[Worker %d] Task %s (Status: %s)", id, task.ID, task.Status)

		ctx := context.WithValue(context.Background(), "task_id", task.ID)
		ctx, cancel := context.WithCancel(ctx)
		wm.Register(task.ID, cancel)

		lockFunc := func(repoName string) (bool, error) {
			ok, err := store.TryLockRepo(repoName, task.ID)
			if ok { store.UpdateTaskRepo(task.ID, repoName) }
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

		if res.RepoName != "" { store.UnlockRepo(res.RepoName) }

		if err != nil {
			if ctx.Err() == context.Canceled {
				log.Printf("🚫 Task %s was stopped by user.", task.ID)
			} else {
				sendToSlack(slackWebhook, fmt.Sprintf("❌ *Task %s 실패*: %v", task.ID, err))
				store.UpdateTaskStatus(task.ID, storage.StatusFailed, "", err.Error())
			}
		} else if res.WaitingApproval {
			store.UpdateTaskStatus(task.ID, storage.StatusWaitingApproval, "", "")
			sendToSlack(slackWebhook, fmt.Sprintf("⏳ *Task %s 검증 완료*: 승인이 필요합니다.", task.ID))
		} else {
			sendToSlack(slackWebhook, fmt.Sprintf("✅ *Task %s 성공!*\n📍 *Repo*: %s\n🔗 *PR*: %s", task.ID, res.RepoName, res.PRURL))
			store.UpdateTaskStatus(task.ID, storage.StatusCompleted, res.PRURL, "")

			for _, chainReq := range res.ChainTasks {
				b, _ := json.Marshal(chainReq)
				newToken, _ := store.CreateTask(string(b))
				sendToSlack(slackWebhook, fmt.Sprintf("🔗 *연쇄 작업 발견!* (%s): %s 레포지토리 수정 예약됨.", newToken.ID, chainReq.TargetRepo))
			}
		}
	}
}

func main() {
	log.Println("🚀 ACS (Auto-Coder Swarm) Starting (Hybrid Architecture)")

	// 1. Environmental Configuration
	dbPath := getEnv("SWARM_DB_PATH", "./swarm.db")
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	natsURL := getEnv("NATS_URL", "nats://localhost:4222")
	oracleURL := getEnv("ORACLE_URL", "http://localhost:8005")
	ckhURL := getEnv("CKH_URL", "http://localhost:8007") // Default CKH URL
	amqpURL := getEnv("AMQP_URL", "amqp://guest:guest@192.168.120.54:5672/")
	masterRepos := getEnv("MASTER_REPOS_PATH", "/home/cnf/projects/code-insight-engine/repos")
	workspaceBase := getEnv("WORKSPACE_BASE_PATH", "/tmp")
	templatesPath := getEnv("TEMPLATES_PATH", "./web/templates")
	slackWebhook := getEnv("SLACK_WEBHOOK_URL", "")
	listenAddr := getEnv("LISTEN_ADDR", ":8006")

	// 2. Messaging (NATS JetStream) & Cache (Redis)
	mb, err := bus.NewMessageBus(natsURL)
	if err != nil { log.Fatalf("❌ Message Bus init failed: %v", err) }
	defer mb.Close()

	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})

	store, err := storage.NewStorage(dbPath, rdb)
	if err != nil { log.Fatalf("❌ DB init failed: %v", err) }
	store.ResetRunningToPending()

	wm := worker.NewManager()
	sm := stream.NewManager(store)
	agent.GlobalStream = &StreamAdapter{manager: sm}
	agent.GlobalStorage = store

	// 3. Orchestration Layer
	ic := insightclient.NewClient(oracleURL, mb)
	cc := ckhclient.NewClient(ckhURL) // Initialize CKH Client
	wsMgr := workspace.NewLocalManager(workspaceBase, masterRepos)
	gitSvc := gitmgr.NewGitManager()

	primaryModelName := store.GetSetting("primary_model")
	if primaryModelName == "" { primaryModelName = "gemma4:31b" }
	primaryModel := llm.NewRabbitMQModel(primaryModelName, amqpURL)
	reportingSvc := reporting.NewService(store, primaryModel)

	sg := security.NewGuardrail(&security.SecretScanner{}, &security.StaticAnalysisScanner{})
	orc := orchestrator.NewSwarmOrchestrator(ic, cc, wsMgr, gitSvc, store, sg)

	// 4. Worker Management
	workerCount := 3
	for w := 1; w <= workerCount; w++ {
		go taskWorker(w, orc, store, wm, slackWebhook)
	}

	// 5. Web Interface & API
	mux := http.NewServeMux()
	handler := api.NewSwarmHandler(store, wm, reportingSvc, ic)
	handler.RegisterRoutes(mux)
	dashHandler := web.NewDashboardHandler(store, wm, sm, templatesPath)
	dashHandler.RegisterRoutes(mux)

	mux.Handle("GET /metrics", promhttp.Handler())

	log.Printf("📡 ACS API & Dashboard listening on %s", listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, mux))
}
