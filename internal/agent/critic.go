package agent

import (
	"context"
	"fmt"
	"google.golang.org/adk/model"
	"strings"
)

type CriticAgent struct {
	llm model.LLM
}

func NewCriticAgent(m model.LLM) *CriticAgent {
	return &CriticAgent{llm: m}
}

func (a *CriticAgent) Name() string {
	return "Critic"
}

func (a *CriticAgent) Process(ctx context.Context, input string) (string, error) {
	prompt := fmt.Sprintf("You are the Swarm Critic.\n"+
		"Your goal is to perform a critical architecture and risk review of the proposed changes.\n\n"+
		"CRITICAL AUDIT POINTS:\n"+
		"1. SIDE EFFECTS: Will this change break unrelated modules?\n"+
		"2. ARCHITECTURE: Does this follow established design patterns?\n"+
		"3. SECURITY: Are there any credential leaks or logic vulnerabilities?\n"+
		"4. RISK: Is this a high-risk change that requires extra caution?\n\n"+
		"WHAT IS NOT A RISK (do not flag these):\n"+
		"- Style, naming, or formatting preferences.\n"+
		"- Hypotheticals: \"could be misused\", \"depending on how it is called\".\n"+
		"- Missing tests or docs, unless the change removes existing ones.\n"+
		"- Anything you cannot point to with a concrete file and line.\n\n"+
		"MANDATORY RULES:\n"+
		"1. Output 'APPROVED' if the changes are safe and well-architected.\n"+
		"2. Only if you found a concrete risk, start with 'RISK_DETECTED:' and **name the\n"+
		"   file and line**. Without a file:line it is not a risk — output APPROVED instead.\n"+
		"3. A reviewer who always finds something is worse than no reviewer.\n\n"+
		"[Proposed Changes & Context]\n%s", input)

	return CallLLM(ctx, a.llm, a.Name(), prompt)
}

func (a *CriticAgent) IsApproved(resp string) bool {
	upper := strings.ToUpper(resp)
	return strings.Contains(upper, "APPROVED") && !strings.Contains(upper, "RISK_DETECTED")
}
