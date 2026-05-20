package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"google.golang.org/adk/model"
	"strings"
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
Your goal is to extract a structured code modification plan from the Oracle's analysis.

MANDATORY RULES:
1. Identify the EXACT repository name from the analysis.
2. Identify the specific file paths and what needs to be changed.
3. Output ONLY a valid JSON object. Do not include any conversational text.

[Target JSON Format]
{
  "repo_name": "repository-name",
  "changes": [
    {
      "file_path": "path/to/file.ext",
      "description": "Brief summary of change",
      "instructions": "Technical details for the Coder agent"
    }
  ]
}

[Oracle Analysis]
` + oracleAnalysis

	return CallLLM(ctx, a.llm, a.Name(), prompt)
}

func (a *PlannerAgent) ParsePlan(raw string) (Plan, error) {
	jsonStr := raw
	if start := strings.Index(raw, "{"); start != -1 {
		if end := strings.LastIndex(raw, "}"); end != -1 {
			jsonStr = raw[start : end+1]
		}
	}

	var plan Plan
	if err := json.Unmarshal([]byte(jsonStr), &plan); err != nil {
		return Plan{}, fmt.Errorf("failed to parse plan JSON: %w", err)
	}

	if plan.RepoName == "" || plan.RepoName == "not_specified" {
		return plan, fmt.Errorf("planner failed to identify target repository")
	}

	return plan, nil
}
