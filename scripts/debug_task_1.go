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
	fmt.Printf("Task #1 Status: %s\n", task.Status)
	fmt.Printf("Task #1 Error: %s\n", task.ErrorLog)
	
	logs, _ := store.GetLogs(1)
	fmt.Println("--- Latest 5 Logs ---")
	start := len(logs) - 5
	if start < 0 { start = 0 }
	for i := start; i < len(logs); i++ {
		fmt.Printf("[%s] %s: %s\n", logs[i].CreatedAt.Format("15:04:05"), logs[i].Stage, logs[i].Message)
	}
}
