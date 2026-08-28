package orchestrator

import (
	"testing"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
)

func plan(paths ...string) agent.Plan {
	p := agent.Plan{}
	for _, f := range paths {
		p.Changes = append(p.Changes, agent.FileChange{FilePath: f})
	}
	return p
}

// 저장소 전체를 돌리면 DB·네트워크를 타는 테스트까지 걸려 오래 걸리고,
// 우리가 건드리지도 않은 곳에서 실패해 엉뚱한 자가치유가 돈다.
func TestChangedDirsScopesToTouchedPackages(t *testing.T) {
	got := changedDirs(plan(
		"internal/domain/tag.go",
		"internal/domain/tag_test.go",
		"internal/handler/blog.go",
	))
	if len(got) != 2 || got[0] != "internal/domain" || got[1] != "internal/handler" {
		t.Fatalf("got %v", got)
	}
}

// 뿌리에 있는 파일은 `./.../...` 이 되어 전체를 돌린다. 그건 피한다.
func TestChangedDirsSkipsRepoRoot(t *testing.T) {
	if got := changedDirs(plan("main.go", "go.mod")); len(got) != 0 {
		t.Errorf("뿌리 파일에 경로가 잡혔다: %v", got)
	}
}

// 상위로 빠져나가는 경로는 저장소 밖을 돌린다.
func TestChangedDirsRejectsEscapes(t *testing.T) {
	if got := changedDirs(plan("../other/x.go", "/etc/y.go")); len(got) != 0 {
		t.Errorf("저장소 밖 경로가 잡혔다: %v", got)
	}
}

// 같은 계획이면 같은 명령이 나와야 한다 — 순서가 흔들리면 재현이 안 된다.
func TestChangedDirsIsStable(t *testing.T) {
	a := changedDirs(plan("b/x.go", "a/y.go", "b/z.go"))
	b := changedDirs(plan("b/z.go", "a/y.go", "b/x.go"))
	if len(a) != 2 || a[0] != "a" || a[1] != "b" {
		t.Fatalf("정렬이 안 됐다: %v", a)
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("순서가 입력에 따라 달라진다: %v vs %v", a, b)
		}
	}
}
