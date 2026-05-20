package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
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
			// No pending tasks, sleep and try again
			time.Sleep(2 * time.Second)
			continue
		}

		log.Printf("[Worker %d] Processing Persistent Task #%d: %s", id, task.ID, task.UserRequest)
		
		// Mark as running
		store.UpdateTaskStatus(task.ID, storage.StatusRunning, "", "")

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		err = orc.RunTask(ctx, task.UserRequest)
		cancel()

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

	dbPath := os.ExpandEnv("/Users/bae/projects/auto-coder-swarm/swarm.db")
	store, err := storage.NewStorage(dbPath)
	if err != nil {
		log.Fatalf("❌ Failed to init storage: %v", err)
	}

	// Recovery: Reset tasks that were stuck in 'RUNNING' status during crash
	if err := store.ResetRunningToPending(); err != nil {
		log.Printf("⚠️ Recovery warning: %v", err)
	}

	ollamaModel := llm.NewOllamaModel("gemma4:31b", "http://localhost:11434")
	ic := insightclient.NewClient("http://localhost:8005")
	wsMgr := workspace.NewLocalManager("/tmp")
	gitSvc := gitmgr.NewGitManager()
	orc := orchestrator.NewSwarmOrchestrator(ic, wsMgr, gitSvc, ollamaModel)

	numWorkers := 2 
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
		
		fmt.Fprintf(w, "Task #%d queued: %s\nCheck logs for progress.", task.ID, query)
	})

	log.Println("📡 Swarm listening for requests on :8006")
	log.Fatal(http.ListenAndServe(":8006", nil))
}
