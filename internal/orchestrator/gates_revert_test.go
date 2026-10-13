package orchestrator

import (
	"strings"
	"testing"
)

// 되돌림은 관문마다 다르다 — 엉뚱한 것을 고쳤으면 버리고, 계약이 한두 군데
// 틀렸으면 맞게 한 것까지 버리지 않는다. 한 곳으로 모으면서 이 구분이
// 조용히 사라지기 쉽다.
func TestGateRevertRules(t *testing.T) {
	src := readSource(t, "gates.go")

	revert := map[string]bool{
		"NOT_THE_WORK":               true,
		"NOTHING_NEW":                true,
		"ALIGNMENT_BLOCKED":          true,
		"STATE_HAS_NO_WAY_TO_CHANGE": false,
		"STATE_HAS_NO_HOME":          false,
		"PROTO_BROKEN":               false,
	}

	for label, want := range revert {
		i := strings.Index(src, `"`+label+`"`)
		if i < 0 {
			t.Errorf("%s 관문이 없다", label)
			continue
		}
		// 다음 관문 앞까지만 본다.
		block := src[i:]
		if j := strings.Index(block[1:], "label:"); j > 0 {
			block = block[:j]
		}
		if got := strings.Contains(block, "revert: true"); got != want {
			t.Errorf("%s: 되돌림 %v 여야 하는데 %v", label, want, got)
		}
	}

	// handOver 가 그 칸을 실제로 본다.
	h := readSource(t, "handover.go")
	if !strings.Contains(h, "if g.revert {") || !strings.Contains(h, `"checkout", "."`) {
		t.Error("handOver 가 revert 를 안 본다")
	}
}
