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
	// Force reset ALL tasks to PENDING
	err = store.DB.Model(&storage.SwarmTask{}).Where("1=1").Update("status", storage.StatusPending).Error
	if err != nil {
		log.Fatal(err)
	}
	log.Println("EMERGENCY RESET: All tasks are now PENDING.")
}
