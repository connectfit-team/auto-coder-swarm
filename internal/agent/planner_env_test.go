package agent

import "testing"

func TestParsePlanUnwrapsEnvelope(t *testing.T) {
	a := &PlannerAgent{}
	raw := `{"response":{"repo_name":"cms","changes":[{"file_path":"cms/src/lib/utils/workstamp.ts","description":"d","instructions":"i"}]}}`
	plan, err := a.ParsePlanWithRepo(raw, "cms")
	if err != nil {
		t.Fatalf("봉투를 못 벗겼다: %v", err)
	}
	if len(plan.Changes) != 1 {
		t.Fatalf("변경 1개여야 한다, got %d", len(plan.Changes))
	}
	// 저장소 이름 접두사는 떼야 한다 — 작업공간에는 cms/ 폴더가 없다.
	if got := plan.Changes[0].FilePath; got != "src/lib/utils/workstamp.ts" {
		t.Errorf("경로 = %q, want src/lib/utils/workstamp.ts", got)
	}
}

func TestParsePlanPlainStillWorks(t *testing.T) {
	a := &PlannerAgent{}
	raw := "```json\n{\"repo_name\":\"cms\",\"changes\":[{\"file_path\":\"src/a.ts\"}]}\n```"
	plan, err := a.ParsePlanWithRepo(raw, "")
	if err != nil {
		t.Fatalf("평범한 계획을 못 읽었다: %v", err)
	}
	if plan.Changes[0].FilePath != "src/a.ts" || plan.RepoName != "cms" {
		t.Errorf("잘못 읽었다: %+v", plan)
	}
}
