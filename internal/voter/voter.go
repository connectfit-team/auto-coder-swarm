package voter

import (
	"context"
	"log"
	"strings"
	"sync"

	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

type MultiModelVoter struct {
	models []model.LLM
}

func NewMultiModelVoter(models ...model.LLM) *MultiModelVoter {
	return &MultiModelVoter{models: models}
}

type VoteResult struct {
	Winner     string
	Conflicting bool
	Details    []string
}

func (v *MultiModelVoter) Vote(ctx context.Context, agentName, prompt string) (VoteResult, error) {
	var wg sync.WaitGroup
	results := make([]string, len(v.models))
	errors := make([]error, len(v.models))

	for i, m := range v.models {
		wg.Add(1)
		go func(idx int, llm model.LLM) {
			defer wg.Done()
			req := &model.LLMRequest{
				Contents: []*genai.Content{{Role: "user", Parts: []*genai.Part{{Text: prompt}}}},
			}
			it := llm.GenerateContent(ctx, req, false)
			var text string
			for resp, err := range it {
				if err != nil {
					errors[idx] = err
					return
				}
				for _, p := range resp.Content.Parts {
					text += p.Text
				}
			}
			results[idx] = strings.TrimSpace(text)
		}(i, m)
	}
	wg.Wait()

	// Simple consensus: Check if all results are similar or identify the majority
	// For code planning, we look for structural similarity or pick the most detailed one if they agree on target
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

	res := VoteResult{
		Winner:      winner,
		Conflicting: maxCount < len(v.models),
		Details:     results,
	}

	log.Printf("[Voter] [%s] Voting complete. Agreement: %d/%d", agentName, maxCount, len(v.models))
	return res, nil
}
