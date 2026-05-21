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

	logs, _ := store.GetLogs(2)
	fmt.Println("--- Logs for Task #2 ---")
	for _, l := range logs {
		fmt.Printf("[%s] %s: %s\n", l.CreatedAt.Format("15:04:05"), l.Stage, l.Message)
	}
}
