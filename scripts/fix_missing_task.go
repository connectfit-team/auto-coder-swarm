package main

import (
	"log"
	"github.com/connectfit-team/auto-coder-swarm/internal/storage"
)

func main() {
	store, err := storage.NewStorage("swarm.db")
	if err != nil {
		log.Fatal(err)
	}
	// Force reset task 1 to PENDING if it's stuck in RUNNING without a worker
	err = store.UpdateTaskStatus(1, storage.StatusPending, "", "Resetting stuck task")
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Task #1 reset to PENDING.")
}
