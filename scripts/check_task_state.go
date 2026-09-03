//go:build ignore

// 일회성 스크립트다. `go build ./...` 대상에서 뺀다 — 한 디렉터리에 func main
// 이 여럿이면 패키지가 통째로 컴파일되지 않아, **저장소 전체의 빌드·vet·test
// 가 막힌다.** 그동안 그래서 `./cmd/... ./internal/...` 로 우회하고 있었다.
//
// 그대로 돌릴 수 있다:  go run scripts/<파일>.go

package main

import (
	"fmt"
	"log"
	"github.com/connectfit-team/auto-coder-swarm/internal/storage"
)

func main() {
	store, err := storage.NewStorage("", "swarm.db", nil)
	if err != nil {
		log.Fatal(err)
	}
	task, _ := store.GetTaskByID(1)
	fmt.Printf("Task #1 Status: %s, Repo: %s\n", task.Status, task.RepoName)
	
	var count int64
	store.DB.Model(&storage.RepoLock{}).Count(&count)
	fmt.Printf("Active Repo Locks: %d\n", count)
}
