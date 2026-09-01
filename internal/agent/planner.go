package agent

import (
	"context"
	"fmt"
	"google.golang.org/adk/model"
	"regexp"
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

// **JSON 이 아니라 줄 단위로 받는다.**
//
// 9B 는 JSON 문자열 안에서 따옴표를 못 지킨다. 실측으로 `"instructions": "
// 1. `+"`work_date`"+` 변수 ( `+"`"+`"..."` 같은 것을 내서 파싱이 깨졌고, 세 후보가 다
// 깨져 작업이 통째로 실패했다. 백틱·따옴표·줄바꿈이 섞인 한국어 설명을
// JSON 에 담게 하는 것 자체가 무리다.
//
// 줄 단위 형식은 따옴표가 몇 개든 상관이 없다. 예전 JSON 도 계속 읽는다.
func (a *PlannerAgent) BuildPrompt(oracleAnalysis string) string {
	return `You are the Swarm Planner.
Your goal is to extract a code modification plan from the Oracle's analysis.

아래 형식 그대로 내라. 다른 말은 쓰지 마라. JSON 으로 쓰지 마라.

REPO: <저장소 이름>
FILE: <고칠 파일 경로 — 저장소 안의 실제 경로>
WHY: <무엇이 왜 잘못됐는지 한 줄>
HOW: <어떻게 고칠지. 여러 줄 써도 된다>
END

고칠 파일이 여럿이면 FILE 부터 END 까지를 반복해라.
따옴표·백틱·줄바꿈을 마음대로 써도 된다 — 형식이 깨지지 않는다.

[Oracle Analysis]
` + oracleAnalysis
}

// 한 덩이의 시작. END 는 있으면 좋고 없어도 된다 — 모델이 잘 빠뜨린다.
var planFileHeadRe = regexp.MustCompile(`(?mi)^[ \t]*FILE:[ \t]*(.+?)[ \t]*$`)
var planRepoRe = regexp.MustCompile(`(?mi)^[ \t]*REPO:[ \t]*(.+?)[ \t]*$`)

// parseLinePlan 은 줄 단위 계획을 읽는다. 형식이 아니면 nil 을 준다.
//
// **END 를 요구하지 않는다.** 처음에는 FILE 부터 END 까지를 한 덩이로 봤는데,
// 모델이 END 를 빼먹으면 전부 못 읽고 "JSON 파싱 실패" 로 죽었다. 다음 FILE
// 이나 글 끝까지를 한 덩이로 본다.
func parseLinePlan(raw string) *Plan {
	heads := planFileHeadRe.FindAllStringSubmatchIndex(raw, -1)
	if len(heads) == 0 {
		return nil
	}
	plan := &Plan{}
	if m := planRepoRe.FindStringSubmatch(raw); m != nil {
		plan.RepoName = strings.TrimSpace(m[1])
	}
	for i, h := range heads {
		path := strings.TrimSpace(raw[h[2]:h[3]])
		bodyStart := h[1]
		bodyEnd := len(raw)
		if i+1 < len(heads) {
			bodyEnd = heads[i+1][0]
		}
		body := raw[bodyStart:bodyEnd]
		// 다음 덩이의 REPO: 줄이 딸려 오면 잘라 낸다.
		if m := planRepoRe.FindStringIndex(body); m != nil {
			body = body[:m[0]]
		}
		body = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(body), "END"))

		why := sectionOf(body, "WHY")
		how := sectionOf(body, "HOW")
		if how == "" {
			how = why
		}
		if path == "" {
			continue
		}
		plan.Changes = append(plan.Changes, FileChange{
			FilePath:     path,
			Description:  why,
			Instructions: how,
		})
	}
	if len(plan.Changes) == 0 {
		return nil
	}
	return plan
}

// sectionOf 는 WHY:/HOW: 뒤의 내용을 다음 표시가 나올 때까지 준다.
func sectionOf(body, label string) string {
	re := regexp.MustCompile(`(?ms)^\s*` + label + `:\s*(.*?)(?:^\s*(?:WHY|HOW|FILE|REPO):|\z)`)
	m := re.FindStringSubmatch(body)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m[1])
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
	// 줄 단위 형식이 먼저다. JSON 은 옛 프롬프트·다른 모델을 위해 남겨 둔다.
	if p := parseLinePlan(raw); p != nil && len(p.Changes) > 0 {
		for i, c := range p.Changes {
			p.Changes[i].FilePath = trimRepoPrefix(c.FilePath, p.RepoName)
		}
		return *p, nil
	}

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
