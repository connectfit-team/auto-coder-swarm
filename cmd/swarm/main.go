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
	"github.com/connectfit-team/auto-coder-swarm/internal/workspace"
)

type Task struct {
	UserRequest string
	ResponseCh  chan error
}

func worker(id int, orc *orchestrator.SwarmOrchestrator, tasks <-chan Task) {
	for t := range tasks {
		log.Printf("[Worker %d] Processing task: %s", id, t.UserRequest)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		err := orc.RunTask(ctx, t.UserRequest)
		cancel()
		t.ResponseCh <- err
	}
}

func main() {
	log.Println("🚀 Auto-Coder Swarm Starting (Phase 3: Step 9 Concurrency Ready)")

	ollamaModel := llm.NewOllamaModel("gemma4:31b", "http://localhost:11434")
	ic := insightclient.NewClient("http://localhost:8005")
	wsMgr := workspace.NewLocalManager("/tmp")
	gitSvc := gitmgr.NewGitManager()
	orc := orchestrator.NewSwarmOrchestrator(ic, wsMgr, gitSvc, ollamaModel)

	taskQueue := make(chan Task, 100)
	numWorkers := 2 
	for w := 1; w <= numWorkers; w++ {
		go worker(w, orc, taskQueue)
	}

	http.HandleFunc("/request", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		if query == "" {
			http.Error(w, "query is required", http.StatusBadRequest)
			return
		}

		respCh := make(chan error)
		taskQueue <- Task{UserRequest: query, ResponseCh: respCh}
		
		fmt.Fprintf(w, "Task queued: %s\nWaiting for results...", query)
		go func() {
			err := <-respCh
			if err != nil {
				log.Printf("❌ Task failed: %v", err)
			} else {
				log.Printf("✅ Task completed: %s", query)
			}
		}()
	})

	log.Println("📡 Swarm listening for requests on :8006")
	log.Fatal(http.ListenAndServe(":8006", nil))
}
