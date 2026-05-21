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
	fmt.Println("--- All Tasks ---")
	for _, t := range tasks {
		fmt.Printf("ID: %d, Status: %s, Repo: %s, Request: %s\n", t.ID, t.Status, t.RepoName, t.UserRequest)
	}

	locks, _ := store.GetNextPendingTask() // Just to see if it's there
	if locks != nil {
		fmt.Printf("Next Pending: #%d\n", locks.ID)
	}
}
