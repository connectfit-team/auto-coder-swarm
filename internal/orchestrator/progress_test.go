package orchestrator

import "testing"

// 나아지는 동안은 이어 가고, 안 나아지면 멈춘다.
func TestHealProgressKeepsGoingWhileImproving(t *testing.T) {
	t.Setenv("SWARM_HEAL_STALL", "2")
	t.Setenv("SWARM_HEAL_MAX", "20")
	p := &healProgress{}

	// 17 → 11 → 6 → 2 : 계속 줄어드는 동안은 멈추지 않는다.
	for _, n := range []int{17, 11, 6, 2} {
		goOn, why := p.step(errsOf(n))
		if !goOn {
			t.Fatalf("오류 %d개로 줄고 있는데 멈췄다: %s", n, why)
		}
	}
	if p.rounds != 4 {
		t.Errorf("회차가 %d다", p.rounds)
	}
}

func TestHealProgressStopsWhenStalled(t *testing.T) {
	t.Setenv("SWARM_HEAL_STALL", "2")
	t.Setenv("SWARM_HEAL_MAX", "20")
	p := &healProgress{}
	p.step(errsOf(5))
	// 안 줄었다 — 한 번은 더 준다(내용은 달라야 고리로 안 본다).
	if goOn, _ := p.step(shift(errsOf(5), "a")); !goOn {
		t.Error("한 번 안 줄었다고 바로 멈췄다")
	}
	goOn, why := p.step(shift(errsOf(5), "b"))
	if goOn {
		t.Error("두 번 이어서 안 줄었는데 계속한다")
	}
	if why == "" {
		t.Error("멈춘 까닭을 안 남긴다")
	}
}

// 같은 오류 묶음이 다시 나오면 고리다 — 회차 상한보다 먼저 잡아야 한다.
func TestHealProgressStopsOnLoop(t *testing.T) {
	t.Setenv("SWARM_HEAL_STALL", "9")
	t.Setenv("SWARM_HEAL_MAX", "99")
	p := &healProgress{}
	p.step(errsOf(9))
	p.step(errsOf(4))
	goOn, why := p.step(errsOf(9)) // 처음 것과 같다
	if goOn {
		t.Error("같은 오류로 도는데 계속한다 — 무한루프가 된다")
	}
	if why == "" {
		t.Error("고리라고 말해 주지 않는다")
	}
}

// 순서만 바뀐 것은 "달라졌다" 가 아니다.
func TestErrorFingerprintIgnoresOrder(t *testing.T) {
	a := []string{"x.go:1:1: undefined: A", "y.go:2:2: undefined: B"}
	b := []string{"y.go:2:2: undefined: B", " x.go:1:1: undefined: A "}
	if fingerprintErrors(a) != fingerprintErrors(b) {
		t.Error("순서와 빈칸만 다른 것을 다른 오류로 본다 — 고리를 못 잡는다")
	}
}

// 마지막 울타리. 나아지는 동안에도 넘지 않는다.
func TestHealProgressHasCeiling(t *testing.T) {
	t.Setenv("SWARM_HEAL_STALL", "99")
	t.Setenv("SWARM_HEAL_MAX", "3")
	p := &healProgress{}
	n := 50
	for i := 0; i < 10; i++ {
		goOn, _ := p.step(errsOf(n))
		n--
		if !goOn {
			if p.rounds != 3 {
				t.Errorf("회차 상한 3인데 %d회에 멈췄다", p.rounds)
			}
			return
		}
	}
	t.Error("회차 상한이 없다 — 영원히 돈다")
}

// 경고와 꾸러미 머리는 세지 않는다.
func TestBuildErrorLines(t *testing.T) {
	out := "# github.com/x/y\n" +
		"internal/a.go:12:3: undefined: Foo\n" +
		"\n" +
		"[WARN] something unrelated\n" +
		"internal/b.go:4:5: cannot use x (type int) as string\n" +
		"warning: deprecated\n"
	got := buildErrorLines(out)
	if len(got) != 2 {
		t.Errorf("오류를 %d개로 셌다: %v", len(got), got)
	}
}

func errsOf(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = "f.go:1:1: undefined: N" + string(rune('a'+i%26))
	}
	return out
}

func shift(errs []string, tag string) []string {
	out := make([]string, len(errs))
	for i, e := range errs {
		out[i] = e + tag
	}
	return out
}
