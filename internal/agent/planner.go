package agent

import (
	"context"
	"encoding/json"
	"fmt"
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

func (a *PlannerAgent) BuildPrompt(oracleAnalysis string) string {
	return `You are the Swarm Planner. 
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
}

func (a *PlannerAgent) BuildRefinePrompt(oracleAnalysis, originalPlan, criticism string) string {
	return fmt.Sprintf(`You are the Swarm Planner (Refinement Mode).
You previously proposed a plan, but the Swarm Critic identified some issues.
Improve your plan based on the criticism while staying consistent with the Oracle's analysis.

MANDATORY RULES:
1. Output ONLY a valid JSON object.
2. Ensure ALL criticisms are addressed.

[Oracle Analysis]
%s

[Original Plan]
%s

[Swarm Critic Feedback]
%s`, oracleAnalysis, originalPlan, criticism)
}

func (a *PlannerAgent) Process(ctx context.Context, oracleAnalysis string) (string, error) {
	return CallLLM(ctx, a.llm, a.Name(), a.BuildPrompt(oracleAnalysis))
}

func (a *PlannerAgent) Refine(ctx context.Context, oracleAnalysis, originalPlan, criticism string) (string, error) {
	return CallLLM(ctx, a.llm, a.Name(), a.BuildRefinePrompt(oracleAnalysis, originalPlan, criticism))
}

func (a *PlannerAgent) ParsePlan(raw string) (Plan, error) {
	jsonStr := ExtractJSON(raw)

	var plan Plan
	if err := json.Unmarshal([]byte(jsonStr), &plan); err != nil {
		return Plan{}, fmt.Errorf("failed to parse plan JSON: %w", err)
	}

	return plan, nil
}

// ParsePlanWithRepo 는 요청이 이미 저장소를 밝힌 경우를 함께 본다.
//
// 계획에 repo_name 이 빠졌다고 작업을 죽이면 안 된다 — 부르는 쪽이 이미
// target_repo 를 줬는데 모델이 그걸 다시 안 적었을 뿐이다. 실제로 그 이유로
// 마지막 시도가 통째로 버려졌다.
func (a *PlannerAgent) ParsePlanWithRepo(raw, fallbackRepo string) (Plan, error) {
	plan, err := a.ParsePlan(raw)
	if err != nil {
		return plan, err
	}
	if plan.RepoName == "" || plan.RepoName == "not_specified" {
		plan.RepoName = fallbackRepo
	}
	if plan.RepoName == "" {
		return plan, fmt.Errorf("대상 저장소를 알 수 없다 (계획에도 요청에도 없다)")
	}
	return plan, nil

	return plan, nil
}
