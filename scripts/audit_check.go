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

	tasks, err := store.GetAllTasks()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("--- Recent Tasks ---")
	for _, t := range tasks {
		fmt.Printf("ID: %d, Status: %s, Repo: %s, Request: %s\n", t.ID, t.Status, t.RepoName, t.UserRequest)
	}
}
