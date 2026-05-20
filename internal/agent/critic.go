package agent

import (
	"context"
	"fmt"
	"google.golang.org/adk/model"
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

func (a *CriticAgent) Process(ctx context.Context, plan string) (string, error) {
	prompt := fmt.Sprintf("You are the Swarm Logic Critic.\n"+
		"Analyze the following proposed code modification plan and identify potential flaws, edge cases, or risks.\n"+
		"Be constructive but strict. If the plan is perfect, say 'PERFECT'.\n\n"+
		"[Proposed Plan]\n%s", plan)

	return CallLLM(ctx, a.llm, a.Name(), prompt)
}
