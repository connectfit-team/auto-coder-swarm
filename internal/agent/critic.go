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
		"MANDATORY RULES:\n"+
		"1. Output 'APPROVED' if the changes are safe and well-architected.\n"+
		"2. If risks are detected, provide detailed feedback starting with 'RISK_DETECTED:'.\n\n"+
		"[Proposed Changes & Context]\n%s", input)

	return CallLLM(ctx, a.llm, a.Name(), prompt)
}

func (a *CriticAgent) IsApproved(resp string) bool {
	upper := strings.ToUpper(resp)
	return strings.Contains(upper, "APPROVED") && !strings.Contains(upper, "RISK_DETECTED")
}
