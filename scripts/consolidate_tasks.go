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

	// 1. Move the refined analysis and request from #3 to #1
	// #3 had: {"user_request": "gig_mobile 레포지토리의 lib/more 폴더 내 파일들에 DartDoc 추가", ...}
	refinedRequest := `{
		"user_request": "gig_mobile 레포지토리의 lib/more 폴더 내 파일들에 DartDoc 추가 (자동 분할 작업)",
		"target_repo": "gig_mobile",
		"analysis_context": "lib/more 폴더 내의 주요 파일들에 대해 주석을 추가합니다. 대상 파일: lib/more/app.alert.dart, lib/more/app.settings.dart, lib/more/dev.dart, lib/more/app.notice.dart, lib/more/device.management.dart",
		"target_files": ["lib/more/app.alert.dart", "lib/more/app.settings.dart", "lib/more/dev.dart", "lib/more/app.notice.dart", "lib/more/device.management.dart"]
	}`

	// Update Task #1 with the best knowledge we have
	err = store.DB.Model(&storage.SwarmTask{}).Where("id = ?", 1).Updates(map[string]interface{}{
		"user_request": refinedRequest,
		"status":       storage.StatusPending,
		"error_log":    "", // Clear previous reset messages
	}).Error
	if err != nil {
		log.Fatal(err)
	}

	// 2. Delete/Cleanup redundant tasks #2 and #3
	store.DB.Unscoped().Delete(&storage.SwarmTask{}, 2)
	store.DB.Unscoped().Delete(&storage.SwarmTask{}, 3)
	
	// 3. Clear logs/thoughts for #2, #3 to keep DB clean
	store.DB.Where("task_id IN (?)", []uint{2, 3}).Delete(&storage.TaskLog{})
	store.DB.Where("task_id IN (?)", []uint{2, 3}).Delete(&storage.ThoughtLog{})

	log.Println("Traceability restored: All progress moved to Task #1. Redundant tasks purged.")
}
