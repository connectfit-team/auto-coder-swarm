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
	fmt.Printf("Task #1 Status: %s, Repo: %s\n", task.Status, task.RepoName)
	
	var count int64
	store.DB.Model(&storage.RepoLock{}).Count(&count)
	fmt.Printf("Active Repo Locks: %d\n", count)
}
