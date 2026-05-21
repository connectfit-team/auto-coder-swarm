package main

import (
	"fmt"
	"log"
	"github.com/connectfit-team/auto-coder-swarm/internal/storage"
)

func main() {
	store, err := storage.NewStorage("swarm.db")
	if err != nil {
		log.Fatal(err)
	}

	tasks, _ := store.GetAllTasks()
	for _, t := range tasks {
		if t.Status == storage.StatusRunning || t.ID == 3 {
			logs, _ := store.GetLogs(t.ID)
			fmt.Printf("--- Logs for Task #%d (Status: %s) ---\n", t.ID, t.Status)
			for _, l := range logs {
				fmt.Printf("[%s] %s: %s\n", l.CreatedAt.Format("15:04:05"), l.Stage, l.Message)
			}
		}
	}
}
