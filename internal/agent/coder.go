package agent

import (
	"context"
	"google.golang.org/adk/model"
)

type CoderAgent struct {
	llm model.LLM
}

func NewCoderAgent(m model.LLM) *CoderAgent {
	return &CoderAgent{llm: m}
}

func (a *CoderAgent) Name() string {
	return "Coder"
}

func (a *CoderAgent) Process(ctx context.Context, instructions string) (string, error) {
	prompt := `You are the Swarm Coder. 
Modify the following code based on the technical instructions. 
Provide the FULL file content after modification.

[Instructions]
` + instructions

	return CallLLM(ctx, a.llm, prompt)
}
