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
var reDigits = regexp.MustCompile(`\d+`)

// pickContractAmong 은 닿은 계약이 여럿일 때 하나를 고른다.
//
// 기계가 후보와 근거를 다 뽑아 놓았으므로 남은 것은 그중 하나를 고르는 물음
// 하나뿐이다. 닫힌 물음이라 지어낼 여지가 없다. 세 번 묻고 과반일 때만 정한다.
// 후보가 하나여도 묻는다. 그 하나가 스스로 「조회에는 쓰지 마라」 라고 적어
// 둔 계약일 수 있고, 그때 「어느 것도 아니다」 라고 답할 길이 있어야 한다.
func (t *taskContext) pickContractAmong(order []string, evidence map[string]string, missing []string) (picked, why string, rejected bool) {
	// **한 번에 하나만 묻는다.** 후보가 많으면 번호 하나를 고르라는 물음이
	// 너무 커서 과반이 나지 않는다 — 실측으로 17개에서 세 번 물어도
	// 정해지지 않았다. 먼저 「이 계약이 맞나?」 로 줄인다.
	if len(order) > shortlistFrom {
		kept := t.shortlistContracts(order, evidence, missing)
		switch len(kept) {
		case 0:
			return "", fmt.Sprintf("계약 후보 %d 가운데 맞다고 한 것이 하나도 없다", len(order)), true
		case 1:
			return kept[0], fmt.Sprintf("계약 후보 %d 가운데 하나씩 물어 %s 만 남았다", len(order), kept[0]), false
		}
		order = kept
	}
	prompt := contractPickPrompt(order, evidence, missing)

	const rounds = 3
	answers := make([]int, 0, rounds)
	for i := 0; i < rounds; i++ {
		answers = append(answers, t.askOwnerIndex(prompt, order))
	}

	pick, votes, decided := majorityIndex(answers)
	switch {
	case decided && pick == 0:
		// **고르지 못한 것과 다르다.** 모델이 「이 가운데 없다」 고 말한 것이다.
		return "", fmt.Sprintf("계약 후보 %d 가운데 어느 것도 아니라고 %d/%d표로 답했다", len(order), votes, rounds), true
	case !decided:
		return "", fmt.Sprintf("계약 후보 %d 가운데 어느 것인지 세 번 물어도 정해지지 않았다", len(order)), false
	}
	picked = order[pick-1]
	return picked, fmt.Sprintf("계약 후보 %d 가운데 %s 를 골랐다(%d/%d표)", len(order), picked, votes, rounds), false
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

// majorityIndex 는 과반을 넘긴 답을 준다.
//
// 「0 이 과반」(어느 것도 아니다)과 「과반이 없다」(못 정했다)를 갈라 준다.
// 둘을 섞으면, 모델이 경고를 읽고 거절했는데도 부르는 쪽이 「못 정했으니
// 따라간 것을 쓰자」 로 넘어간다.
func majorityIndex(answers []int) (pick, votes int, decided bool) {
	tally := map[int]int{}
	for _, a := range answers {
		tally[a]++
	}
	best, count := -1, 0
	for idx, n := range tally {
		if n > count || (n == count && idx < best) {
			best, count = idx, n
		}
	}
	if best < 0 || count*2 <= len(answers) {
		return 0, count, false
	}
	return best, count, true
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
		if n, ok := lineAsNumber(s); ok {
			if n < 0 || n > len(order) {
				return 0
			}
			return n
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

// lineAsNumber 는 그 줄이 번호 하나를 말하는지 본다.
//
// 「3」·「3번」·「답: 3」 은 번호이고, 경로나 문장은 아니다. 숫자가 여럿이거나
// 경로가 섞였거나 줄이 길면 번호로 보지 않는다 — 문장에서 숫자를 주우면
// 「v1」 의 1 이 번호가 된다.
func lineAsNumber(s string) (int, bool) {
	if strings.ContainsAny(s, "/\\") || len([]rune(s)) > 12 {
		return 0, false
	}
	m := reDigits.FindAllString(s, -1)
	if len(m) != 1 {
		return 0, false
	}
	n, err := strconv.Atoi(m[0])
	if err != nil {
		return 0, false
	}
	return n, true
}
