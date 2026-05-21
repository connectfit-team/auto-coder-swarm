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
	err = store.UpdateContextState(1, "Test State Update")
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Manual state update successful.")
}
