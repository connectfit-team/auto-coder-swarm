package agent

import (
	"strings"
	"testing"
)

// 떨어진 대목을 붙여서 보여 주면 모델이 이어진 줄로 믿는다.
//
// 실제로 5줄과 한참 아래를 이어 붙여 SEARCH 를 만들었고, 그런 줄은 원문에
// 없으니 고치기가 통째로 실패했다(W-31838).
func TestRelevantRegionsMarksGaps(t *testing.T) {
	var lines []string
	lines = append(lines, "const alpha = 1;")
	for i := 0; i < 200; i++ {
		lines = append(lines, "// xxxxx")
	}
	lines = append(lines, "const omega = 2;")
	got := relevantRegions(strings.Join(lines, "\n"), "alpha 와 omega 를 고쳐라")

	if got == "" {
		t.Fatal("관련 대목을 하나도 못 골랐다")
	}
	if !strings.Contains(got, "alpha") || !strings.Contains(got, "omega") {
		t.Fatalf("두 대목을 다 보여 주지 않았다:\n%s", got)
	}
	if !strings.Contains(got, "이어진 줄이 아니다") {
		t.Errorf("생략을 분명히 적지 않았다:\n%s", got)
	}
	for _, line := range strings.Split(got, "\n") {
		if strings.TrimSpace(line) == "..." {
			t.Errorf("옛 표시가 남아 있다:\n%s", got)
			break
		}
	}
}
