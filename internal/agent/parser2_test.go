package agent

import (
	"strings"
	"testing"
)

func TestParseLinePlanWithoutEND(t *testing.T) {
	a := &PlannerAgent{}
	// 모델이 실제로 낸 모양 — END 가 없고 REPO 가 덩이마다 반복된다.
	raw := "REPO: cms\nFILE: src/a.ts\nWHY: 하나\nHOW:\n- 첫째\n- 둘째\n\n\n" +
		"REPO: cms\nFILE: src/b.ts\nWHY: 둘\nHOW:\n- 셋째\n\n"
	plan, err := a.ParsePlanWithRepo(raw, "")
	if err != nil {
		t.Fatalf("END 없는 계획을 못 읽었다: %v", err)
	}
	if len(plan.Changes) != 2 {
		t.Fatalf("변경 %d개, want 2: %+v", len(plan.Changes), plan.Changes)
	}
	if plan.Changes[0].FilePath != "src/a.ts" || plan.Changes[1].FilePath != "src/b.ts" {
		t.Errorf("경로가 틀렸다: %+v", plan.Changes)
	}
	if !strings.Contains(plan.Changes[0].Instructions, "둘째") {
		t.Errorf("HOW 여러 줄을 못 읽었다: %q", plan.Changes[0].Instructions)
	}
	if strings.Contains(plan.Changes[0].Instructions, "REPO") {
		t.Errorf("다음 덩이가 딸려 왔다: %q", plan.Changes[0].Instructions)
	}
}

func TestParseLinePlanStillReadsEND(t *testing.T) {
	a := &PlannerAgent{}
	raw := "REPO: cms\nFILE: src/a.ts\nWHY: 하나\nHOW: 고친다\nEND\n"
	plan, err := a.ParsePlanWithRepo(raw, "")
	if err != nil || len(plan.Changes) != 1 {
		t.Fatalf("END 있는 계획을 못 읽었다: %v %+v", err, plan)
	}
	if strings.Contains(plan.Changes[0].Instructions, "END") {
		t.Errorf("END 가 지시문에 남았다: %q", plan.Changes[0].Instructions)
	}
}
