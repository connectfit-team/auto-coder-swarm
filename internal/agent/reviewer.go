package agent

import (
	"context"
	"fmt"
	"google.golang.org/adk/model"
	"strings"
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
	prompt := fmt.Sprintf("You are the Swarm Reviewer.\n"+
		"Your goal is to verify the code modifications made by the Coder agent.\n\n"+
		"MANDATORY RULES:\n"+
		"1. Output 'APPROVED' if the changes are correct and follow conventions.\n"+
		"2. If there are issues, provide feedback starting with 'FEEDBACK:'.\n\n"+
		"[Code Changes]\n%s", diff)

	return CallLLM(ctx, a.llm, a.Name(), prompt)
}

func (a *ReviewerAgent) IsApproved(resp string) bool {
	upper := strings.ToUpper(resp)
	return strings.Contains(upper, "APPROVED") && !strings.Contains(upper, "FEEDBACK")
}
