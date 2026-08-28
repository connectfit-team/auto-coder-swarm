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
		"2. Only if you found a concrete problem **in this diff**, start with 'FEEDBACK:'\n"+
		"   and name the file and line. If you cannot point to a file and line that\n"+
		"   appears in the diff above, output APPROVED instead.\n"+
		"3. Do not discuss code that is not in the diff.\n"+
		"4. A reviewer who always finds something is worse than no reviewer.\n\n"+
		"[Code Changes]\n%s", diff)

	return CallLLM(ctx, a.llm, a.Name(), prompt)
}

func (a *ReviewerAgent) IsApproved(resp string) bool {
	upper := strings.ToUpper(resp)
	return strings.Contains(upper, "APPROVED") && !strings.Contains(upper, "FEEDBACK")
}
