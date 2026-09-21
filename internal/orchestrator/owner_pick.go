package orchestrator

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
)

// 답이 번호 하나일 때만 번호로 받는다.
//
// 아무 데서나 첫 정수를 주우면 경로로 답한 회차에서 「v1」 의 1 이 번호가
// 된다 — 범위 안이라 걸러지지도 않고, 같은 실수를 세 번 되풀이해 다수결까지
// 붙는다.
var reOnlyNumber = regexp.MustCompile(`^(\d+)\s*[.)번]?\s*$`)

// pickContractAmong 은 닿은 계약이 여럿일 때 하나를 고른다.
//
// 기계가 후보와 근거를 다 뽑아 놓았으므로 남은 것은 그중 하나를 고르는 물음
// 하나뿐이다. 닫힌 물음이라 지어낼 여지가 없다. 세 번 묻고 과반일 때만 정한다.
func (t *taskContext) pickContractAmong(order []string, evidence map[string]string, missing []string) (string, string) {
	if len(order) == 1 {
		return order[0], "후보가 하나뿐이라 묻지 않았다"
	}
	prompt := contractPickPrompt(order, evidence, missing)

	const rounds = 3
	answers := make([]int, 0, rounds)
	for i := 0; i < rounds; i++ {
		answers = append(answers, t.askOwnerIndex(prompt, order))
	}

	pick, votes := majorityIndex(answers)
	if pick == 0 {
		return "", fmt.Sprintf("계약 후보 %d 가운데 어느 것인지 세 번 물어도 정해지지 않았다", len(order))
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
「계약 — 근거 (펴낸 저장소)」 이고, 그 계약에 적힌 말이 있으면 함께 적었다.

%s

번호 하나만 적어라. 어느 것도 아니면 0 을 적어라. 다른 말은 쓰지 마라.`,
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
func (t *taskContext) askOwnerIndex(prompt string, order []string) int {
	ctx, cancel := context.WithTimeout(t.ctx, stateCheckTimeout)
	defer cancel()
	raw, err := agent.CallLLM(ctx, t.primaryLLM, "StateOwnerPick", prompt)
	if err != nil {
		return 0
	}
	return parseOwnerPick(raw, order)
}

// parseOwnerPick 은 답에서 고른 번호를 읽는다.
//
// 첫 줄 **전체**가 번호일 때만 번호로 받고, 그 밖에는 답 안에 목록의 계약
// 경로가 딱 하나 들어 있을 때만 그것으로 받는다. 둘 다 아니면 0 이다.
func parseOwnerPick(raw string, order []string) int {
	for _, line := range strings.Split(raw, "\n") {
		s := strings.TrimSpace(line)
		if s == "" {
			continue
		}
		s = strings.TrimSpace(strings.Trim(s, "`'\"*"))
		if m := reOnlyNumber.FindStringSubmatch(s); m != nil {
			i, err := strconv.Atoi(m[1])
			if err != nil || i < 0 || i > len(order) {
				return 0
			}
			return i
		}
		break
	}
	hits, idx := 0, 0
	for i, k := range order {
		if k != "" && strings.Contains(raw, k) {
			hits++
			idx = i + 1
		}
	}
	if hits == 1 {
		return idx
	}
	return 0
}
