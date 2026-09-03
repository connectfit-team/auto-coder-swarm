//go:build ignore

// 일회성 스크립트다. `go build ./...` 대상에서 뺀다 — 한 디렉터리에 func main
// 이 여럿이면 패키지가 통째로 컴파일되지 않아, **저장소 전체의 빌드·vet·test
// 가 막힌다.** 그동안 그래서 `./cmd/... ./internal/...` 로 우회하고 있었다.
//
// 그대로 돌릴 수 있다:  go run scripts/<파일>.go

package main
import (
	"log"
	"github.com/connectfit-team/auto-coder-swarm/internal/storage"
)
func main() {
	store, _ := storage.NewStorage("", "swarm.db", nil)
	store.DB.Model(&storage.SwarmTask{}).Where("id = 1").Update("status", storage.StatusPending)
	log.Println("Task #1 reset to PENDING for Brain-Detection test.")
}
