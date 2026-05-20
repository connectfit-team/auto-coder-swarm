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
			
			// Use agent.CallLLM for logging and streaming
			specificAgentName := fmt.Sprintf("%s (%s)", agentName, modelName)
			text, err := agent.CallLLM(ctx, llm, specificAgentName, prompt)
			if err != nil {
				errors[idx] = err
				return
			}
			results[idx] = strings.TrimSpace(text)
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

	// Fallback if no consensus
	if winner == "" && len(results) > 0 {
		winner = results[0]
	}

	res := VoteResult{
		Winner:      winner,
		Conflicting: maxCount < len(v.models),
		Details:     results,
	}

	log.Printf("[Voter] [%s] Voting complete. Winner agreement: %d/%d", agentName, maxCount, len(v.models))
	return res, nil
}
