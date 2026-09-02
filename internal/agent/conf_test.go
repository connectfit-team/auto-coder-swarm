package agent

import (
	"strings"
	"testing"
)

func TestPlanFromDefectReportPrefersHighConfidence(t *testing.T) {
	// 분석이 여러 파일에서 원인을 뽑아 오지만 확신은 하나뿐인 모양.
	analysis := "**`src/lib/components/workplace/CalendarView.svelte`**\n" +
		"원인: const endOfMonth = moment(currentDate).endOf('month')\n" +
		"이유: 말일 계산이 어긋날 수 있음\n" +
		"확신: 낮음\n" +
		"**`src/lib/server/workplace/workplace.ts`**\n" +
		"원인: lt: new Date(new Date().getFullYear(), new Date().getMonth() + 1, 0)\n" +
		"이유: 해당 월의 0일을 만들어 말일이 빠진다\n" +
		"확신: 높음\n" +
		"**`src/lib/server/db/schedule.ts`**\n" +
		"원인: if (!from && !to) return undefined;\n" +
		"이유: 범위가 없을 수 있음\n" +
		"확신: 낮음\n"

	got := PlanFromDefectReport(analysis)
	if len(got) != 1 {
		t.Fatalf("확신 높은 것 1개만 남아야 한다, got %d: %+v", len(got), got)
	}
	if got[0].FilePath != "src/lib/server/workplace/workplace.ts" {
		t.Errorf("경로 = %q", got[0].FilePath)
	}
	if !strings.Contains(got[0].Instructions, "getMonth() + 1, 0)") {
		t.Errorf("문제의 줄이 빠졌다: %q", got[0].Instructions)
	}
}

func TestPlanFromDefectReportFallsBackWhenNoneConfident(t *testing.T) {
	// 확신이 높은 것이 하나도 없으면 아무것도 안 하는 것보다 낫다.
	analysis := "**`a.ts`**\n원인: x\n이유: y\n확신: 낮음\n" +
		"**`b.ts`**\n원인: z\n이유: w\n확신: 낮음\n"
	got := PlanFromDefectReport(analysis)
	if len(got) != 2 {
		t.Errorf("전부 낮음이면 다 쓴다, got %d", len(got))
	}
}

func TestPlanFromDefectReportWithoutConfidenceLine(t *testing.T) {
	// 확신 줄이 없는 옛 형식도 계속 읽는다.
	analysis := "**`a.ts`**\n원인: x\n이유: y\n"
	if got := PlanFromDefectReport(analysis); len(got) != 1 {
		t.Errorf("옛 형식을 못 읽었다: %+v", got)
	}
}
