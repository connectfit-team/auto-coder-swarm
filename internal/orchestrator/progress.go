package orchestrator

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// 횟수로 끊지 않고 **나아지는 동안** 이어 간다.
//
// 시도를 셋으로 묶어 두었더니, 오류가 열일곱 개에서 열한 개로 줄고 있는
// 도중에 "최대 시도 초과" 로 끝났다. 사용자의 기준은 이렇다 —
// **"느려도 괜찮아(최대 몇일도 괜찮음) 확실한 작동만 보장하면 돼."**
//
// 그러면 무엇으로 멈추나. 횟수보다 강한 조건이 있다.
//
//   - **나아지지 않으면 멈춘다.** 오류가 줄지 않은 회차가 이어지면 그
//     치유기는 그 문제를 못 푸는 것이다.
//   - **같은 자리를 두 번 돌면 멈춘다.** 오류 묶음이 앞서 본 것과 똑같으면
//     고리다. 이것이 무한루프를 막는 진짜 잠금이다 — 횟수가 아니라 상태다.
//   - 그 위에 마지막 울타리로 회차 상한을 둔다. 기계가 멈춘 것을 못 알아채는
//     경우(외부 서비스가 매번 다른 오류를 내는 등)를 위한 것이다.
//
// 판단은 셈으로 한다. 모델에게 "나아지고 있나" 를 묻지 않는다 — 그 답은
// 스스로 틀릴 수 있고, 틀린 답이 고리를 만든다.

const (
	// 나아지지 않은 회차가 이만큼 이어지면 멈춘다.
	defaultStallRounds = 2
	// 마지막 울타리. 나아지는 동안에도 이 회차를 넘지 않는다.
	defaultMaxRounds = 12
)

// healProgress 는 회차마다 오류가 어떻게 바뀌는지 따라간다.
type healProgress struct {
	rounds  int
	best    int      // 지금까지 가장 적었던 오류 수
	stalled int      // 나아지지 않은 회차가 몇 번 이어졌나
	seen    []string // 앞서 본 오류 묶음의 지문
	history []string // 사람이 볼 진행 곡선
}

func stallRounds() int   { return envInt("SWARM_HEAL_STALL", defaultStallRounds) }
func maxHealRounds() int { return envInt("SWARM_HEAL_MAX", defaultMaxRounds) }

func envInt(name string, def int) int {
	if v := os.Getenv(name); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

// step 은 이번 회차의 오류를 받아 계속할지 정한다.
// 돌려주는 것은 (계속하나, 멈추는 까닭)이다.
func (p *healProgress) step(errs []string) (bool, string) {
	p.rounds++
	n := len(errs)
	fp := fingerprintErrors(errs)

	for _, old := range p.seen {
		if old == fp {
			p.note(n, "앞서 본 것과 같은 오류다")
			return false, fmt.Sprintf("같은 오류로 돌고 있다 (회차 %d, 오류 %d개)", p.rounds, n)
		}
	}
	p.seen = append(p.seen, fp)

	switch {
	case p.rounds == 1:
		p.best = n
		p.note(n, "시작")
	case n < p.best:
		p.note(n, fmt.Sprintf("줄었다 (%d → %d)", p.best, n))
		p.best = n
		p.stalled = 0
	default:
		p.stalled++
		p.note(n, fmt.Sprintf("안 줄었다 (%d회 이어짐)", p.stalled))
	}

	if p.stalled >= stallRounds() {
		return false, fmt.Sprintf("%d회 이어서 나아지지 않았다 (오류 %d개)", p.stalled, n)
	}
	if p.rounds >= maxHealRounds() {
		return false, fmt.Sprintf("회차 상한 %d 에 닿았다 (오류 %d개, 가장 적었을 때 %d개)",
			maxHealRounds(), n, p.best)
	}
	return true, ""
}

func (p *healProgress) note(n int, why string) {
	p.history = append(p.history, fmt.Sprintf("%d회: 오류 %d개 — %s", p.rounds, n, why))
}

// Curve 는 사람이 볼 진행 곡선이다. PR·로그에 그대로 적는다.
func (p *healProgress) Curve() string { return strings.Join(p.history, "\n") }

// fingerprintErrors 는 오류 묶음의 지문이다. 순서와 중복은 무시한다 —
// 같은 오류가 순서만 바뀌어 오는 것을 "달라졌다" 고 읽으면 고리를 못 잡는다.
func fingerprintErrors(errs []string) string {
	norm := make([]string, 0, len(errs))
	for _, e := range errs {
		t := strings.TrimSpace(e)
		if t != "" {
			norm = append(norm, t)
		}
	}
	sort.Strings(norm)
	sum := sha256.Sum256([]byte(strings.Join(norm, "\n")))
	return hex.EncodeToString(sum[:8])
}
