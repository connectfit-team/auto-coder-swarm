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

	thoughts, err := store.GetThoughts(5)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("--- Thoughts for Task #5 (%d) ---\n", len(thoughts))
	for _, t := range thoughts {
		fmt.Printf("[%s] %s: %s\n", t.CreatedAt.Format("15:04:05"), t.AgentName, t.Message)
	}
}
