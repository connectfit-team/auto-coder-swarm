package voter

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
	"google.golang.org/adk/model"
)

type MultiModelVoter struct {
	models []model.LLM
}

func NewMultiModelVoter(models ...model.LLM) *MultiModelVoter {
	return &MultiModelVoter{models: models}
}

type VoteResult struct {
	Winner      string
	Conflicting bool
	Details     []string
}

type NamedLLM interface {
	Name() string
}

func extractJSON(raw string) string {
	return agent.ExtractJSON(raw)
}

func (v *MultiModelVoter) Vote(ctx context.Context, agentName, prompt string) (VoteResult, error) {
	var wg sync.WaitGroup
	results := make([]string, len(v.models))
	errors := make([]error, len(v.models))

	for i, m := range v.models {
		wg.Add(1)
		go func(idx int, llm model.LLM) {
			defer wg.Done()

			modelName := "Unknown"
			if n, ok := llm.(NamedLLM); ok {
				modelName = n.Name()
			}

			specificAgentName := fmt.Sprintf("%s (%s)", agentName, modelName)
			text, err := agent.CallLLM(ctx, llm, specificAgentName, prompt)
			if err != nil {
				errors[idx] = err
				return
			}
			// **원문을 그대로 둔다.**
			//
			// 여기서 JSON 만 뽑아 저장했다. 그래서 계획을 줄 단위 형식으로
			// 받도록 바꾼 뒤에도 투표를 거치면 형식이 통째로 부서졌고,
			// HOW 안의 코드 예시에 있던 중괄호가 JSON 으로 오인돼
			// "invalid character 'g'" 로 죽었다. 정규화는 합의를 세는 데만
			// 쓰고, 돌려주는 것은 원문이어야 한다.
			results[idx] = text
		}(i, m)
	}
	wg.Wait()

	// 합의는 정규화한 열쇠로 센다. 같은 답을 띄어쓰기만 다르게 낸 것을
	// 다른 답으로 세면 합의가 영영 안 생긴다.
	counts := make(map[string]int)
	firstRaw := make(map[string]string)
	for _, res := range results {
		if res == "" {
			continue
		}
		k := consensusKey(res)
		counts[k]++
		if _, ok := firstRaw[k]; !ok {
			firstRaw[k] = res
		}
	}

	winnerKey := ""
	maxCount := 0
	for k, count := range counts {
		if count > maxCount {
			maxCount = count
			winnerKey = k
		}
	}
	winner := firstRaw[winnerKey]

	if winner == "" {
		// Collect errors if all models failed
		var errMsgs []string
		for _, err := range errors {
			if err != nil {
				errMsgs = append(errMsgs, err.Error())
			}
		}
		if len(errMsgs) > 0 {
			return VoteResult{}, fmt.Errorf("all voter models failed: %s", strings.Join(errMsgs, "; "))
		}
		return VoteResult{}, fmt.Errorf("all voter models returned empty responses")
	}

	res := VoteResult{
		Winner:      winner,
		Conflicting: maxCount < len(v.models),
		Details:     results,
	}

	log.Printf("[Voter] [%s] Voting complete. Agreement: %d/%d. Winner Length: %d", agentName, maxCount, len(v.models), len(winner))
	return res, nil
}

// consensusKey 는 답을 견주기 위한 열쇠다.
//
// JSON 이 들어 있으면 그 부분만, 아니면 공백을 접은 원문을 쓴다. 어느 쪽이든
// **세는 데만** 쓰고 돌려주는 것은 원문이다.
func consensusKey(raw string) string {
	// **진짜 JSON 일 때만** 그것으로 견준다. 줄 단위 계획의 HOW 안에 코드
	// 예시가 있으면 그 중괄호가 JSON 으로 오인돼, 서로 다른 계획이 같은
	// 답으로 세어진다.
	if j := extractJSON(raw); j != "" && json.Valid([]byte(j)) {
		return strings.Join(strings.Fields(j), " ")
	}
	return strings.Join(strings.Fields(raw), " ")
}
