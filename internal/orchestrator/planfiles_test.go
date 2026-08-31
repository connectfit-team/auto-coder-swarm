package orchestrator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
)

func TestSplitByRepoReality(t *testing.T) {
	repo := t.TempDir()
	os.MkdirAll(filepath.Join(repo, "src/lib/server/db"), 0o755)
	os.WriteFile(filepath.Join(repo, "src/lib/server/db/attendance.ts"), []byte("x"), 0o644)

	changes := []agent.FileChange{
		{FilePath: "src/lib/server/db/attendance.ts"}, // 있다
		{FilePath: "src/lib/server/db/workplace.ts"},  // 없지만 폴더는 있다 — 새 파일
		{FilePath: "statistics/internal/event-synchronizer/workplace.go"}, // 다른 저장소
		{FilePath: "/etc/passwd"},        // 저장소 밖
		{FilePath: "../other/thing.ts"},  // 저장소 밖
		{FilePath: ""},                   // 빈 경로
	}
	kept, dropped := splitByRepoReality(repo, changes)
	if len(kept) != 2 {
		t.Errorf("살릴 것은 2개다, got %d: %+v", len(kept), kept)
	}
	if len(dropped) != 4 {
		t.Errorf("뺄 것은 4개다, got %d: %v", len(dropped), dropped)
	}
}
