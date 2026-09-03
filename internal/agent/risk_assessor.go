package agent

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/adk/model"
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

func (a *RiskAssessorAgent) BuildPrompt(diff string) string {
	return fmt.Sprintf("You are the Swarm Risk Assessor.\n"+
		"Analyze the following code changes and identify potential side effects or risks to other modules or services.\n\n"+
		"MANDATORY RULES:\n"+
		"1. Output 'SAFE' if the changes have minimal impact and no hidden risks.\n"+
		"2. If there are potential risks (e.g., breaking API, performance regression), list them starting with 'RISK:'.\n\n"+
		"[Code Changes]\n%s", diff)
}

func (a *RiskAssessorAgent) Process(ctx context.Context, diff string) (string, error) {
	return CallLLM(ctx, a.llm, a.Name(), a.BuildPrompt(diff))
}

func (a *RiskAssessorAgent) IsSafe(resp string) bool {
	upper := strings.ToUpper(resp)
	return strings.Contains(upper, "SAFE") && !strings.Contains(upper, "RISK:")
}
