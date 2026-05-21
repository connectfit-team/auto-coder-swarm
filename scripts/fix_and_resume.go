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

	// 1. Force Task #1 to PENDING and clear errors
	err = store.UpdateTaskStatus(1, storage.StatusPending, "", "Resuming with fixed context")
	if err != nil {
		log.Fatal(err)
	}

	// 2. Inject explicit RepoName into the request context
	refinedRequest := `{
		"user_request": "gig_mobile 레포지토리의 lib/more 폴더 내 파일들에 DartDoc 주석 추가",
		"target_repo": "gig_mobile",
		"analysis_context": "[CRITICAL] 대상 레포지토리: gig_mobile\n분석 결과: lib/more 폴더 내의 핵심 파일들(app.alert.dart, app.settings.dart, dev.dart 등)에 대해 DartDoc 표준(///)을 준수하는 주석을 추가해야 합니다.",
		"target_files": ["lib/more/app.alert.dart", "lib/more/app.settings.dart", "lib/more/dev.dart", "lib/more/app.notice.dart", "lib/more/device.management.dart"]
	}`
	
	err = store.DB.Model(&storage.SwarmTask{}).Where("id = ?", 1).Update("user_request", refinedRequest).Error
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Task #1 context fixed and reset to PENDING.")
}
