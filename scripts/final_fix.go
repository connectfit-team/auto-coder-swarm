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

	// 1. Force Task #1 to PENDING with a bullet-proof explicit request
	refinedRequest := `{
		"user_request": "gig_mobile 레포지토리의 lib/more 폴더 내 모든 파일에 DartDoc 주석 추가",
		"target_repo": "gig_mobile",
		"analysis_context": "[MANDATORY] REPO_NAME: gig_mobile. TASK: Add DartDoc (///) to all files in lib/more. Target files are known and verified in the workspace.",
		"target_files": ["lib/more/app.alert.dart", "lib/more/app.settings.dart", "lib/more/dev.dart", "lib/more/app.notice.dart", "lib/more/device.management.dart"]
	}`
	
	store.DB.Model(&storage.SwarmTask{}).Where("id = ?", 1).Updates(map[string]interface{}{
		"user_request": refinedRequest,
		"status":       storage.StatusPending,
		"context_state": "[INIT] Task consolidated and repository identified as gig_mobile.",
		"error_log":    "",
	})

	log.Println("Task #1 context re-hardened and reset.")
}
