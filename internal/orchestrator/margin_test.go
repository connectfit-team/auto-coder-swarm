package orchestrator

import "testing"

// 이 모델은 계약을 줄 세우지 못한다. 경로만 주면 다섯 계약이 모두 3점이었고,
// 근거 글을 붙이면 엉뚱한 계약이 정답보다 높았다. 한 점 차이로 고르면 그
// 흔들림이 그대로 답이 된다.
func TestScoreNeedsAClearMargin(t *testing.T) {
	if scoreMargin < 2 {
		t.Errorf("점수 차이를 %d 점만 요구한다 — 흔들림이 답이 된다", scoreMargin)
	}
	order := []string{"a", "b"}
	cases := []struct {
		name   string
		scores map[string]int
		want   string
	}{
		{"뚜렷하다", map[string]int{"a": 5, "b": 1}, "a"},
		{"한 점 차이", map[string]int{"a": 3, "b": 2}, ""},
		{"딱 두 점", map[string]int{"a": 4, "b": 2}, "a"},
	}
	for _, c := range cases {
		best, top, second, _ := bestByScore(order, c.scores)
		got := best
		if top-second < scoreMargin {
			got = ""
		}
		if got != c.want {
			t.Errorf("%s: %q, 기대 %q (%d점 vs %d점)", c.name, got, c.want, top, second)
		}
	}
}
