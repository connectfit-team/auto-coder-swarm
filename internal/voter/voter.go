package voter

import (
	"context"
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
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start != -1 && end != -1 && end > start {
		return raw[start : end+1]
	}
	return strings.TrimSpace(raw)
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
			// Normalize by extracting JSON to increase consensus probability
			results[idx] = extractJSON(text)
		}(i, m)
	}
	wg.Wait()

	counts := make(map[string]int)
	for _, res := range results {
		if res != "" {
			counts[res]++
		}
	}

	winner := ""
	maxCount := 0
	for res, count := range counts {
		if count > maxCount {
			maxCount = count
			winner = res
		}
	}

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
