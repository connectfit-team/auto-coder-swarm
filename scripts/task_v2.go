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

	// Create a much more explicit task to avoid any Oracle ambiguity
	req := `{"user_request": "gig_mobile 레포지토리의 lib/more 폴더 아래의 파일들에 DartDoc 주석을 추가해줘. 대상 파일: app.alert.dart, app.settings.dart, dev.dart 등 총 16개 파일.", "target_repo": "gig_mobile", "target_files": ["lib/more/app.alert.dart", "lib/more/app.settings.dart", "lib/more/dev.dart"]}`
	task, err := store.CreateTask(req)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Explicit Task Created: #%d\n", task.ID)
}
