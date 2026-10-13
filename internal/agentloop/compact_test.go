package agentloop

import (
	"context"
	"errors"
	"strings"
	"testing"

	"google.golang.org/genai"
)

func longTurns(n int, each string) []*genai.Content {
	out := []*genai.Content{userTurn("지시와 목표")}
	for i := 0; i < n; i++ {
		out = append(out, modelTurn(each), userTurn(each))
	}
	return out
}

// 창이 차면 **요약해서** 접는다. 그냥 버리면 본 것이 사라져 같은 도구를 또 부른다.
func TestCompactSummarizesInsteadOfDropping(t *testing.T) {
	big := strings.Repeat("조사한 내용이 아주 길다. ", 400)
	history := longTurns(6, big)

	var sawHead bool
	l := &Loop{
		Summarize: func(_ context.Context, text string) (string, error) {
			sawHead = strings.Contains(text, "조사한 내용")
			return "open_binary 로 /bin/true 를 열었다.", nil
		},
	}
	got := l.compact(context.Background(), history)

	if !sawHead {
		t.Error("접을 내용을 요약기에 안 넘겼다")
	}
	if len(got) >= len(history) {
		t.Errorf("줄지 않았다: %d -> %d", len(history), len(got))
	}
	joined := flatten(got)
	if !strings.Contains(joined, "open_binary 로 /bin/true 를 열었다.") {
		t.Error("요약이 궤적에 안 들어갔다")
	}
	if !strings.Contains(joined, "되풀이하지 마라") {
		t.Error("이미 한 것이라고 일러 주지 않는다")
	}
	if !strings.Contains(joined, "지시와 목표") {
		t.Error("맨 앞(지시·목표)을 버렸다")
	}
	if tokensOf(got) > historyBudget() {
		t.Errorf("접고도 예산을 넘는다: %d > %d", tokensOf(got), historyBudget())
	}
}

// 요약이 실패해도 루프를 멈추지 않는다.
func TestCompactFallsBackWhenSummaryFails(t *testing.T) {
	big := strings.Repeat("길다. ", 1500)
	history := longTurns(6, big)
	l := &Loop{Summarize: func(context.Context, string) (string, error) {
		return "", errors.New("모델이 죽었다")
	}}
	got := l.compact(context.Background(), history)
	if len(got) == 0 {
		t.Fatal("통째로 비웠다")
	}
	if tokensOf(got) > historyBudget() {
		t.Errorf("예산을 넘는다: %d", tokensOf(got))
	}
}

// 창에 들어가면 손대지 않는다 — 요약은 공짜가 아니다.
func TestCompactLeavesSmallHistoryAlone(t *testing.T) {
	history := longTurns(2, "짧다")
	called := false
	l := &Loop{Summarize: func(context.Context, string) (string, error) {
		called = true
		return "x", nil
	}}
	got := l.compact(context.Background(), history)
	if called {
		t.Error("작은 궤적인데 요약을 불렀다")
	}
	if len(got) != len(history) {
		t.Errorf("건드렸다: %d -> %d", len(history), len(got))
	}
}
