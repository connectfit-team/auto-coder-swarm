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
	// Force reset ALL stuck tasks to PENDING
	err = store.ResetRunningToPending()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("All tasks reset to PENDING.")
}
