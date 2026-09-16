package orchestrator

import "testing"

// 시도 2가 오류 1개까지 갔는데 시도 3이 37개로 되돌아가고, 남은 것은 시도 3
// 뿐이었다(W-38946). 가장 나았던 회차를 기억해야 이어갈 수 있다.
func TestHealProgressLast(t *testing.T) {
	p := &healProgress{}
	if p.last() != -1 {
		t.Errorf("아직 돈 적 없으면 -1 이어야 한다: %d", p.last())
	}
	p.step([]string{"a", "b", "c"})
	if p.last() != 3 {
		t.Errorf("마지막 회차 오류 수가 틀렸다: %d", p.last())
	}
	p.step([]string{"a"})
	if p.last() != 1 {
		t.Errorf("줄어든 것을 못 읽었다: %d", p.last())
	}
	// 늘어난 회차도 그대로 센다 — 마지막 상태가 워크트리에 남은 것이다.
	p.step([]string{"a", "b", "c", "d", "e"})
	if p.last() != 5 {
		t.Errorf("늘어난 것을 못 읽었다: %d", p.last())
	}
}
