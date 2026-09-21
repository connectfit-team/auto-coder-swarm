package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
)

// 후보가 많으면 한 번에 하나씩 묻는다.
//
// 열일곱 가운데 하나를 고르라는 물음은 너무 크다 — 실측으로 세 번 물어도
// 과반이 나지 않았다. 「이 계약이 맞나?」 는 예·아니오 하나여서 답이
// 흔들리지 않고, 남는 것이 하나면 더 물을 것도 없다.
//
// 값은 물음 수다. 후보 17개면 51번 묻는다. 느린 것은 괜찮다.
const shortlistFrom = 3

// shortlistContracts 는 「이 계약이 맞나?」 를 하나씩 물어 후보를 줄인다.
func (t *taskContext) shortlistContracts(order []string, evidence map[string]string, missing []string) []string {
	var kept []string
	for _, k := range order {
		if t.contractFits(k, evidence[k], missing) {
			kept = append(kept, k)
		}
	}
	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CONTRACT_SHORTLIST",
		fmt.Sprintf("후보 %d 가운데 %d 개가 남았다", len(order), len(kept)),
		strings.Join(kept, ", "), "")
	return kept
}

// contractFits 는 한 계약에 대해 예·아니오를 세 번 묻고 과반으로 정한다.
func (t *taskContext) contractFits(contract, evidence string, missing []string) bool {
	prompt := contractFitPrompt(contract, evidence, missing)
	yes := 0
	const rounds = 3
	for i := 0; i < rounds; i++ {
		if t.asksYes(prompt) {
			yes++
		}
	}
	return yes*2 > rounds
}

// contractFitPrompt 는 예·아니오 하나를 묻는다.
func contractFitPrompt(contract, evidence string, missing []string) string {
	return fmt.Sprintf(`[없어서 못 만드는 것]
%s

[계약]
%s
%s

이 계약에 위 상태를 담는 것이 맞나? 예 또는 아니오 하나만 적어라.
그 계약에 「쓰지 마라」·「앱과 같은 RPC 다」 처럼 쓰지 말라고 적혀 있으면
아니오다. 다른 말은 쓰지 마라.`,
		strings.Join(missing, "\n"), contract, evidence)
}

// asksYes 는 예·아니오 하나를 받는다. 못 읽으면 아니오다.
func (t *taskContext) asksYes(prompt string) bool {
	ctx, cancel := context.WithTimeout(t.ctx, stateCheckTimeout)
	defer cancel()
	raw, err := agent.CallLLM(ctx, t.primaryLLM, "ContractFit", prompt)
	if err != nil {
		return false
	}
	return readsAsYes(raw)
}

// readsAsYes 는 답이 「예」 인지 본다. 애매하면 아니오다.
func readsAsYes(raw string) bool {
	for _, line := range strings.Split(raw, "\n") {
		s := strings.ToLower(strings.TrimSpace(line))
		if s == "" {
			continue
		}
		s = strings.TrimSpace(strings.Trim(s, "`'\"*.,!"))
		switch {
		case strings.HasPrefix(s, "아니") || strings.HasPrefix(s, "no"):
			return false
		case strings.HasPrefix(s, "예") || strings.HasPrefix(s, "yes") || s == "y":
			return true
		}
		return false // 첫 줄이 예·아니오가 아니면 아니오로 본다
	}
	return false
}
