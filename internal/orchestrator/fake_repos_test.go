package orchestrator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/connectfit-team/auto-coder-swarm/internal/workspace"
)

// newFakeRepos 는 이름만 있는 사본 폴더를 만들어 실제 Manager 로 감싼다.
// 이름 검사는 폴더가 있느냐로 하는 것이므로 흉내가 아니라 같은 길이다.
func newFakeRepos(names ...string) *workspace.LocalManager {
	dir, err := os.MkdirTemp("", "acs-repos")
	if err != nil {
		panic(err)
	}
	for _, n := range names {
		os.MkdirAll(filepath.Join(dir, n), 0o755)
	}
	return workspace.NewLocalManager(dir, dir)
}

var _ = testing.Short
