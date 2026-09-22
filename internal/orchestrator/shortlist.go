package orchestrator

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
)

// 후보를 고를 때는 **한 번에 하나씩, 점수로** 묻는다.
//
// 「열일곱 가운데 번호 하나」 는 세 번 물어도 과반이 나지 않았다. 「이 계약이
// 맞나? 예·아니오」 로 쪼갰더니 이번에는 **무엇에든 아니오**라고 답했다 —
// 정답까지 떨어졌다(실측: 정답·오답 모두 3회 전부 아니오).
//
// 점수는 갈린다. 같은 모델에 0~10 을 물으니 정답 계약이 3·3·3, 엉뚱한
// 계약이 0·0·0 과 0·2·0 이었다. 절대값은 낮아도 **차례는 분명하다.**
// 그래서 가장 높은 하나를 고르되, 같은 점수가 둘이면 고르지 않는다.
var reScore = regexp.MustCompile(`\d+`)

const scoreRounds = 3

// scoreContracts 는 후보마다 점수를 매긴다.
func (t *taskContext) scoreContracts(order []string, evidence map[string]string, missing []string) map[string]int {
	out := map[string]int{}
	for _, k := range order {
		out[k] = t.scoreContract(k, evidence[k], missing)
	}
	return out
}

// scoreContract 는 한 계약을 세 번 물어 **가운데 값**을 준다.
// 한 번도 못 읽으면 -1 이다 — 0(전혀 아니다)과 다르다.
func (t *taskContext) scoreContract(contract, evidence string, missing []string) int {
	prompt := contractScorePrompt(contract, evidence, missing)
	var got []int
	for i := 0; i < scoreRounds; i++ {
		if n, ok := t.askScore(prompt); ok {
			got = append(got, n)
		}
	}
	if len(got) == 0 {
		return -1
	}
	sort.Ints(got)
	return got[len(got)/2]
}

// contractScorePrompt 는 숫자 하나를 묻는다.
func contractScorePrompt(contract, evidence string, missing []string) string {
	return fmt.Sprintf(`[없어서 못 만드는 것]
%s

[계약]
%s
%s

이 계약이 위 상태를 담기에 얼마나 알맞은가? 0 부터 10 사이의 숫자 하나만
적어라. 그 계약에 쓰지 말라고 적혀 있으면 0 이다. 다른 말은 쓰지 마라.`,
		strings.Join(missing, "\n"), contract, evidence)
}

// askScore 는 숫자 하나를 받는다. 두 번째 값은 읽었는지다.
func (t *taskContext) askScore(prompt string) (int, bool) {
	ctx, cancel := context.WithTimeout(t.ctx, stateCheckTimeout)
	defer cancel()
	raw, err := agent.CallLLM(ctx, t.primaryLLM, "ContractScore", prompt)
	if err != nil {
		return 0, false
	}
	return parseScore(raw)
}

// parseScore 는 첫 줄에서 0~10 을 읽는다. 그 밖은 못 읽은 것이다.
func parseScore(raw string) (int, bool) {
	for _, line := range strings.Split(raw, "\n") {
		s := strings.TrimSpace(line)
		if s == "" {
			continue
		}
		if strings.ContainsAny(s, "/\\") || len([]rune(s)) > 12 {
			return 0, false
		}
		m := reScore.FindAllString(s, -1)
		if len(m) != 1 {
			return 0, false
		}
		n, err := strconv.Atoi(m[0])
		if err != nil || n < 0 || n > 10 {
			return 0, false
		}
		return n, true
	}
	return 0, false
}

// bestByScore 는 가장 높은 하나를 준다.
//
// 같은 점수가 둘이면 고르지 않는다 — 고르면 차례가 아니라 훑는 순서로
// 정해진다. 모두 0 이면 「어느 것도 아니다」 이고, 모두 못 읽었으면 미결정이다.
func bestByScore(order []string, scores map[string]int) (best string, top, second int, read bool) {
	top, second = -1, -1
	for _, k := range order {
		s, ok := scores[k]
		if !ok || s < 0 {
			continue
		}
		read = true
		switch {
		case s > top:
			second = top
			top, best = s, k
		case s > second:
			second = s
		}
	}
	if top >= 0 && top == second {
		return "", top, second, read
	}
	return best, top, second, read
}
