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

	// Submit a task with analysis context PRE-FILLED to bypass Oracle if needed
	req := `{
		"user_request": "gig_mobile 레포지토리의 lib/more 폴더 내 파일들에 DartDoc 추가",
		"target_repo": "gig_mobile",
		"analysis_context": "lib/more 폴더 내의 주요 파일들에 대해 주석을 추가합니다. 대상 파일: lib/more/app.alert.dart, lib/more/app.settings.dart, lib/more/dev.dart, lib/more/app.notice.dart, lib/more/device.management.dart",
		"target_files": ["lib/more/app.alert.dart", "lib/more/app.settings.dart", "lib/more/dev.dart", "lib/more/app.notice.dart", "lib/more/device.management.dart"]
	}`
	task, err := store.CreateTask(req)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Bypassing Task Created: #%d\n", task.ID)
}
