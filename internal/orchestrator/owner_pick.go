package orchestrator

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
)

var reFirstInt = regexp.MustCompile(`\d+`)

// pickContractAmong 은 닿은 계약이 여럿일 때 하나를 고른다.
//
// 고칠 파일들이 서로 다른 계약을 쓰면 후보가 둘 이상 나온다. 여태는 거기서
// 손을 뗐고("어느 쪽인지 알 수 없다"), 그러면 연쇄가 만들어지지 않아 사람은
// 아무것도 받지 못했다.
//
// 이것은 **고르는 자리**다. 기계가 후보와 근거(파일 → 공장 → 계약)를 이미
// 다 뽑아 놓았으므로 남은 것은 그중 하나를 고르는 물음 하나뿐이다. 물음은
// 닫혀 있다 — 번호 하나여서 지어낼 여지가 없다.
//
// 세 번 묻고 다수결로 정한다. 한 번만 물으면 회차마다 답이 달랐다.
func (t *taskContext) pickContractAmong(order []string, evidence map[string]string, missing []string) (string, string) {
	prompt := contractPickPrompt(order, evidence, missing)

	const rounds = 3
	answers := make([]int, 0, rounds)
	for i := 0; i < rounds; i++ {
		answers = append(answers, t.askOwnerIndex(prompt, len(order)))
	}

	pick, votes := majorityIndex(answers)
	if pick == 0 {
		return "", fmt.Sprintf("고칠 파일들이 서로 다른 계약을 쓴다(%s) — 세 번 물어도 어느 것인지 정해지지 않았다",
			strings.Join(order, ", "))
	}
	picked := order[pick-1]
	return picked, fmt.Sprintf("계약 후보 %d 가운데 %s 를 골랐다(%d/%d표)", len(order), picked, votes, rounds)
}

// contractPickPrompt 는 닫힌 물음을 만든다 — 번호 하나.
func contractPickPrompt(order []string, evidence map[string]string, missing []string) string {
	var lines []string
	for i, o := range order {
		lines = append(lines, fmt.Sprintf("%d) %s — %s", i+1, o, evidence[o]))
	}
	return fmt.Sprintf(`[없어서 못 만드는 것]
%s

이것을 담을 자리를 만들 곳은 아래 계약 가운데 어느 것인가? 각 줄은
「계약 — 어느 파일이 어느 공장을 거쳐 거기에 닿는지 (펴낸 저장소)」 다.

%s

**번호 하나만** 적어라. 어느 것도 아니면 0 을 적어라. 다른 말은 쓰지 마라.`,
		strings.Join(missing, "\n"), strings.Join(lines, "\n"))
}

// majorityIndex 는 과반을 넘긴 번호를 준다. 없거나 0 이 이기면 0 이다.
func majorityIndex(answers []int) (int, int) {
	votes := map[int]int{}
	for _, a := range answers {
		votes[a]++
	}
	best, count := 0, 0
	for idx, n := range votes {
		if idx == 0 {
			continue
		}
		if n > count || (n == count && idx < best) {
			best, count = idx, n
		}
	}
	if best == 0 || count*2 <= len(answers) {
		return 0, count
	}
	return best, count
}

// askOwnerIndex 는 번호 하나를 받는다. 못 읽으면 0 이다.
func (t *taskContext) askOwnerIndex(prompt string, n int) int {
	ctx, cancel := context.WithTimeout(t.ctx, stateCheckTimeout)
	defer cancel()
	raw, err := agent.CallLLM(ctx, t.primaryLLM, "StateOwnerPick", prompt)
	if err != nil {
		return 0
	}
	return parseOwnerIndex(raw, n)
}

// parseOwnerIndex 는 답에서 첫 번호를 읽는다. 범위를 벗어나면 0 이다.
func parseOwnerIndex(raw string, n int) int {
	m := reFirstInt.FindString(raw)
	if m == "" {
		return 0
	}
	i, err := strconv.Atoi(m)
	if err != nil || i < 0 || i > n {
		return 0
	}
	return i
}
