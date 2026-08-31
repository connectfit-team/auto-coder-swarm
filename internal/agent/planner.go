package agent

import (
	"context"
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

	// 봉투에 싸여 오는 일이 있다 — {"response":{"repo_name":…,"changes":[…]}}.
	// 그냥 Unmarshal 하면 **오류 없이** 빈 Plan 이 되고, 그게 "계획에 고칠
	// 파일이 하나도 없다" 로 보고돼 진짜 원인이 가려진다.
	var plan Plan
	if err := unmarshalMaybeWrapped([]byte(jsonStr), &plan); err != nil {
		return Plan{}, fmt.Errorf("failed to parse plan JSON: %w", err)
	}

	// 모델이 경로 앞에 저장소 이름을 붙여 준다 — cms/src/lib/… 처럼.
	// 작업공간 안에는 그런 폴더가 없어서 없는 파일로 걸러진다.
	for i, c := range plan.Changes {
		plan.Changes[i].FilePath = trimRepoPrefix(c.FilePath, plan.RepoName)
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
	// repo_name 이 뒤늦게 정해졌으면 경로도 다시 다듬는다.
	for i, c := range plan.Changes {
		plan.Changes[i].FilePath = trimRepoPrefix(c.FilePath, plan.RepoName)
	}
	return plan, nil
}

// trimRepoPrefix 는 경로 앞에 붙은 저장소 이름을 뗀다.
//
// 모델이 "cms/src/lib/utils/workstamp.ts" 처럼 적어 준다. 작업공간은 이미 그
// 저장소 안이라 cms/ 라는 폴더가 없고, 그대로 두면 없는 파일로 걸러진다.
func trimRepoPrefix(path, repo string) string {
	path = strings.TrimSpace(path)
	if repo == "" {
		return path
	}
	if after, ok := strings.CutPrefix(path, repo+"/"); ok {
		return after
	}
	return path
}
