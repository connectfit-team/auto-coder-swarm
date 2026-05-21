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
	fmt.Println("--- Context State for Task #1 ---")
	fmt.Println(task.ContextState)
}
