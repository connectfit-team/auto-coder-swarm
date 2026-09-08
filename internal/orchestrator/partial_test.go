package orchestrator

import (
	"encoding/json"
	"strings"
	"testing"
)

// S-05 — 반쪽만 고쳐진 채로 빌드로 넘어가면 안 된다.
//
// 흐름 전체를 돌리지 않고, 목록에 적힌 기대가 코드에 실제로 있는지 본다.
// stepExecution 은 코더·워크트리가 필요해 단위 시험으로 부르기 어렵다.
func TestPartialCodingRetries(t *testing.T) {
	var want string
	for _, raw := range scopeProblemsRaw(t) {
		var p struct {
			ID     string `json:"id"`
			Expect string `json:"expect"`
		}
		if json.Unmarshal(raw, &p) == nil && p.ID == "S-05" {
			want = p.Expect
		}
	}
	if want == "" {
		t.Skip("목록에 S-05 가 없다")
	}

	src := readSource(t, "flow_execution.go")
	for _, must := range []string{
		// 못 고친 것이 있으면 계획으로 돌아간다
		"errRetryPlanning",
		// 그 사실을 사람이 볼 수 있게 남긴다
		"CODING_PARTIAL",
		// 횟수는 묶여 있다 — 새 고리를 만들지 않는다
		"attempt < 3",
	} {
		if !strings.Contains(src, must) {
			t.Errorf("S-05: flow_execution.go 에 %q 가 없다 — 기대: %s", must, want)
		}
	}
	// 실패가 있어도 nil 을 돌려주면 빌드로 넘어간다.
	if strings.Contains(src, "t.lastFeedback = \"CODER FAILED:") {
		t.Errorf("S-05: 실패를 되먹임에만 적고 그대로 지나가는 옛 모양이 남아 있다")
	}
}
