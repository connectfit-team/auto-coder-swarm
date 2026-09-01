package agent

import (
	"strings"
	"testing"
)

func TestParseLinePlan(t *testing.T) {
	a := &PlannerAgent{}
	// 따옴표·백틱·줄바꿈이 섞여도 형식이 깨지지 않아야 한다 — JSON 이
	// 깨졌던 바로 그 내용이다.
	raw := "REPO: cms\n" +
		"FILE: src/lib/server/workplace/workplace.ts\n" +
		"WHY: `lt` 에 \"말일 00:00\" 을 줘서 말일 기록이 통째로 빠진다\n" +
		"HOW: new Date(y, m + 1, 0) 을 new Date(y, m + 1, 1) 로 바꾼다.\n" +
		"     29행과 133행 두 곳 모두 고친다.\n" +
		"END\n"
	plan, err := a.ParsePlanWithRepo(raw, "")
	if err != nil {
		t.Fatalf("줄 단위 계획을 못 읽었다: %v", err)
	}
	if plan.RepoName != "cms" {
		t.Errorf("repo = %q", plan.RepoName)
	}
	if len(plan.Changes) != 1 {
		t.Fatalf("변경 %d개", len(plan.Changes))
	}
	c := plan.Changes[0]
	if c.FilePath != "src/lib/server/workplace/workplace.ts" {
		t.Errorf("경로 = %q", c.FilePath)
	}
	if !strings.Contains(c.Description, "말일") {
		t.Errorf("WHY 를 못 읽었다: %q", c.Description)
	}
	if !strings.Contains(c.Instructions, "133행") {
		t.Errorf("HOW 의 여러 줄을 못 읽었다: %q", c.Instructions)
	}
}

func TestParseLinePlanMultipleFiles(t *testing.T) {
	a := &PlannerAgent{}
	raw := `REPO: cms
FILE: a.ts
WHY: 하나
HOW: 고친다
END
FILE: b.ts
WHY: 둘
HOW: 고친다
END`
	plan, err := a.ParsePlanWithRepo(raw, "")
	if err != nil || len(plan.Changes) != 2 {
		t.Fatalf("파일 두 개를 못 읽었다: %v %+v", err, plan)
	}
}

func TestParsePlanStillReadsJSON(t *testing.T) {
	a := &PlannerAgent{}
	raw := "```json\n{\"repo_name\":\"cms\",\"changes\":[{\"file_path\":\"src/a.ts\"}]}\n```"
	plan, err := a.ParsePlanWithRepo(raw, "")
	if err != nil || len(plan.Changes) != 1 || plan.Changes[0].FilePath != "src/a.ts" {
		t.Fatalf("옛 JSON 형식을 못 읽었다: %v %+v", err, plan)
	}
}
