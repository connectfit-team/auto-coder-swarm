package main

import (
	"context"
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

func worker(id int, orc *orchestrator.SwarmOrchestrator, store *storage.Storage) {
	for {
		task, err := store.GetNextPendingTask()
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}

		log.Printf("[Worker %d] Processing Task #%d: %s", id, task.ID, task.UserRequest)
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

		repoName, err := orc.RunTask(ctx, task.UserRequest, lockFunc)
		cancel()

		if lockedRepo != "" {
			store.UnlockRepo(lockedRepo)
		} else if repoName != "" {
			store.UnlockRepo(repoName)
		}

		if err != nil {
			log.Printf("❌ Task #%d failed: %v", task.ID, err)
			store.UpdateTaskStatus(task.ID, storage.StatusFailed, "", err.Error())
		} else {
			log.Printf("✅ Task #%d completed successfully", task.ID)
			store.UpdateTaskStatus(task.ID, storage.StatusCompleted, "Success", "")
		}
	}
}

func main() {
	log.Println("🚀 Auto-Coder Swarm Starting (Phase 4: SQLite Persistence Ready)")

	// Use absolute path for DB to ensure GORM can find it regardless of execution dir
	dbPath := "/home/cnf/projects/auto-coder-swarm/swarm.db"
	store, err := storage.NewStorage(dbPath)
	if err != nil {
		// Try fallback to local directory if absolute path fails for some reason
		log.Printf("⚠️ Absolute path failed, trying local path...")
		store, err = storage.NewStorage("swarm.db")
		if err != nil {
			log.Fatalf("❌ Failed to init storage: %v", err)
		}
	}

	if err := store.ResetRunningToPending(); err != nil {
		log.Printf("⚠️ Recovery warning: %v", err)
	}

	ollamaModel := llm.NewOllamaModel("gemma4:31b", "http://localhost:11434")
	ic := insightclient.NewClient("http://localhost:8005")
	wsMgr := workspace.NewLocalManager("/tmp")
	gitSvc := gitmgr.NewGitManager()
	orc := orchestrator.NewSwarmOrchestrator(ic, wsMgr, gitSvc, ollamaModel)

	numWorkers := 1 
	for w := 1; w <= numWorkers; w++ {
		go worker(w, orc, store)
	}

	http.HandleFunc("/request", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		if query == "" {
			http.Error(w, "query is required", http.StatusBadRequest)
			return
		}
		task, err := store.CreateTask(query)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to create task: %v", err), http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "Task #%d queued: %s", task.ID, query)
	})

	log.Println("📡 Swarm listening for requests on :8006")
	log.Fatal(http.ListenAndServe(":8006", nil))
}
