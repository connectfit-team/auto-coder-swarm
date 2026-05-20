package agent

import (
	"context"
	"google.golang.org/adk/model"
)

type ReviewerAgent struct {
	llm model.LLM
}

func NewReviewerAgent(m model.LLM) *ReviewerAgent {
	return &ReviewerAgent{llm: m}
}

func (a *ReviewerAgent) Name() string {
	return "Reviewer"
}

func (a *ReviewerAgent) Process(ctx context.Context, diff string) (string, error) {
	prompt := `You are the Swarm Reviewer. 
Review the following code changes for quality, convention, and potential risks.
Output "APPROVED" if everything is correct, or provide specific feedback for improvement.

[Changes]
` + diff

	return CallLLM(ctx, a.llm, prompt)
}
