package orchestrator

import (
	"strings"
	"testing"
)

// 시도마다 달라지는 꼬리까지 넣고 세면 같은 이유도 늘 새 것으로 보인다.
func TestDenialReasonIgnoresVaryingTail(t *testing.T) {
	a := denialReason("PROTO: service ConnectService 를 새로 만들었다\n\n[시도 1 의 수정을 되살려 두었다 — 남은 오류 4개]")
	b := denialReason("PROTO: service ConnectService 를 새로 만들었다\n\n[시도 2 의 수정을 되살려 두었다 — 남은 오류 2개]")
	if a != b {
		t.Fatalf("같은 이유가 다르게 세어진다:\n%q\n%q", a, b)
	}
	if denialReason("   \n  ") != "" {
		t.Fatal("빈 되먹임에서 까닭이 나온다")
	}
	long := denialReason(strings.Repeat("가", 400))
	if len([]rune(long)) > 160 {
		t.Fatalf("너무 길다: %d자", len([]rune(long)))
	}
}

func TestRepeatedDenialPicksTheWorst(t *testing.T) {
	tc := &taskContext{denials: map[string]int{"A 때문": 1, "B 때문": 3, "C 때문": 2}}
	reason, n := tc.repeatedDenial()
	if reason != "B 때문" || n != 3 {
		t.Fatalf("가장 많이 되풀이된 것을 못 골랐다: %q %d", reason, n)
	}
	// 같은 횟수면 매번 같은 답이 나와야 한다.
	tie := &taskContext{denials: map[string]int{"나중": 2, "가장먼저": 2}}
	r1, _ := tie.repeatedDenial()
	r2, _ := tie.repeatedDenial()
	if r1 != r2 {
		t.Fatalf("같은 횟수에서 답이 흔들린다: %q vs %q", r1, r2)
	}
}

// 한 번뿐이면 되풀이가 아니다 — 엉뚱한 단정은 없느니만 못하다.
func TestWhyStuckNeedsTwo(t *testing.T) {
	once := &taskContext{denials: map[string]int{"한 번 막혔다": 1}}
	if err := once.whyStuck(); err != nil {
		t.Fatalf("한 번인데 되풀이라고 한다: %v", err)
	}
	twice := &taskContext{denials: map[string]int{"두 번 막혔다": 2}}
	err := twice.whyStuck()
	if err == nil {
		t.Fatal("두 번 막혔는데 아무 말도 없다")
	}
	if !strings.Contains(err.Error(), "두 번 막혔다") || !strings.Contains(err.Error(), "2번") {
		t.Fatalf("까닭도 횟수도 안 적혔다: %v", err)
	}
	// 「최대 시도 초과」 로 가려지면 안 된다.
	if strings.Contains(err.Error(), "최대 시도 초과") {
		t.Fatalf("까닭이 다시 가려졌다: %v", err)
	}
}

func TestWhyStuckEmptyWhenNothingRecorded(t *testing.T) {
	if err := (&taskContext{}).whyStuck(); err != nil {
		t.Fatalf("아무것도 안 셌는데 단정한다: %v", err)
	}
}
