package agent

import (
	"strings"
	"testing"
)

func TestApplyEditBlocksToleratesRepeatedBlock(t *testing.T) {
	// 같은 코드가 두 곳에 있으면 모델은 블록을 두 번 낸다. 첫 블록이 둘 다
	// 고치고 나면 두 번째는 찾을 것이 없다 — 그것 때문에 전체가 버려지면 안 된다.
	orig := "a {\n    lt: new Date(y, m + 1, 0)\n}\nb {\n    lt: new Date(y, m + 1, 0)\n}\n"
	one := "<<<<<<< SEARCH\nlt: new Date(y, m + 1, 0)\n=======\nlt: new Date(y, m + 1, 1)\n>>>>>>> REPLACE"
	raw := one + "\n\n" + one

	got, err := applyEditBlocks(orig, raw)
	if err != nil {
		t.Fatalf("같은 블록을 두 번 냈다고 통째로 버렸다: %v", err)
	}
	if n := strings.Count(got, "m + 1, 1)"); n != 2 {
		t.Errorf("고쳐진 자리 %d개, want 2:\n%s", n, got)
	}
	if strings.Contains(got, "m + 1, 0)") {
		t.Errorf("안 고쳐진 자리가 남았다:\n%s", got)
	}
}

func TestApplyEditBlocksStillFailsIfNothingApplied(t *testing.T) {
	// 하나도 못 고쳤으면 여전히 실패여야 한다 — 조용히 성공으로 넘기면 안 된다.
	orig := "const a = 1;\n"
	raw := "<<<<<<< SEARCH\nzzz not here zzz\n=======\nqqq\n>>>>>>> REPLACE"
	if _, err := applyEditBlocks(orig, raw); err == nil {
		t.Error("아무것도 못 고쳤는데 성공으로 넘겼다")
	}
}
