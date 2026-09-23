package orchestrator

import (
	"strings"
	"testing"
)

// 계약 수정은 대개 대부분 맞고 한두 가지가 틀리다. 통째로 되돌리면 맞게
// 한 것까지 사라져 다음 시도가 처음부터 간다.
func TestProtoGateKeepsWhatWasFixed(t *testing.T) {
	src := readSource(t, "handover.go")
	i := strings.Index(src, "CheckProtoChange(t.repoPath, diff)")
	if i < 0 {
		t.Fatal("계약 관문이 없다")
	}
	// 그 관문 안에서 작업 트리를 되돌리면 안 된다.
	block := src[i:]
	if j := strings.Index(block, "\n\tif bad := CheckToolLeak"); j > 0 {
		block = block[:j]
	}
	if strings.Contains(block, `"checkout", "."`) {
		t.Error("계약 관문이 고친 것을 통째로 되돌린다 — 맞게 한 것까지 사라진다")
	}
	for _, must := range []string{"위에 적힌 자리만 고쳐라", "처음부터 다시 쓰지 마라"} {
		if !strings.Contains(block, must) {
			t.Errorf("되먹임에 %q 가 없다", must)
		}
	}
}
