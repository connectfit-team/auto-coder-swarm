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
	store.SaveSetting("primary_model", "gemma4:31b")
	store.SaveSetting("voter_models", "gemma4:31b,llama3:70b-instruct-q8_0,qwen2.5vl:32b")
	log.Println("Default settings initialized.")
}
