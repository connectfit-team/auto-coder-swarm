package agent

import (
	"context"
	"fmt"
	"google.golang.org/adk/model"
	"strings"
)

type RiskAssessorAgent struct {
	llm model.LLM
}

func NewRiskAssessorAgent(m model.LLM) *RiskAssessorAgent {
	return &RiskAssessorAgent{llm: m}
}

func (a *RiskAssessorAgent) Name() string {
	return "RiskAssessor"
}

func (a *RiskAssessorAgent) Process(ctx context.Context, diff string) (string, error) {
	prompt := fmt.Sprintf("You are the Swarm Risk Assessor.\n"+
		"Analyze the following code changes and identify potential side effects or risks to other modules or services.\n\n"+
		"MANDATORY RULES:\n"+
		"1. Output 'SAFE' if the changes have minimal impact and no hidden risks.\n"+
		"2. If there are potential risks (e.g., breaking API, performance regression), list them starting with 'RISK:'.\n\n"+
		"[Code Changes]\n%s", diff)

	return CallLLM(ctx, a.llm, a.Name(), prompt)
}

func (a *RiskAssessorAgent) IsSafe(resp string) bool {
	upper := strings.ToUpper(resp)
	return strings.Contains(upper, "SAFE") && !strings.Contains(upper, "RISK:")
}
