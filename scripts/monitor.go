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

	task, _ := store.GetTaskByID(1)
	fmt.Printf("Task #1: Status=%s, Repo=%s\n", task.Status, task.RepoName)
	
	logs, _ := store.GetLogs(1)
	fmt.Println("--- Recent Logs ---")
	for i := len(logs)-5; i < len(logs); i++ {
		if i >= 0 {
			fmt.Printf("[%s] %s: %s\n", logs[i].CreatedAt.Format("15:04:05"), logs[i].Stage, logs[i].Message)
		}
	}
}
