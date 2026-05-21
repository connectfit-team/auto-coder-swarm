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
	logs, _ := store.GetLogs(1)
	fmt.Printf("Total Logs for Task #1: %d\n", len(logs))
	if len(logs) > 0 {
		l := logs[len(logs)-1]
		fmt.Printf("Latest Log: Stage=%s, Msg=%s, PromptLen=%d\n", l.Stage, l.Message, len(l.Prompt))
	}
}
