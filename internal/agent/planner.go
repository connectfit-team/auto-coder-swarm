package agent

import (
	"context"
	"google.golang.org/adk/model"
)

type PlannerAgent struct {
	llm model.LLM
}

func NewPlannerAgent(m model.LLM) *PlannerAgent {
	return &PlannerAgent{llm: m}
}

func (a *PlannerAgent) Name() string {
	return "Planner"
}

func (a *PlannerAgent) Process(ctx context.Context, oracleAnalysis string) (string, error) {
	prompt := `You are the Swarm Planner. 
Analyze the following technical analysis from the Oracle and create a structured JSON plan for code modification.
Ensure the plan is realistic and targets specific files.

[Target Format]
{
  "repo_name": "...",
  "changes": [
    {
      "file_path": "...",
      "description": "What to change",
      "instructions": "Specific technical instructions for the Coder agent"
    }
  ]
}

[Oracle Analysis]
` + oracleAnalysis

	return CallLLM(ctx, a.llm, prompt)
}
