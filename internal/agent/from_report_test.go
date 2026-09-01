package agent

import (
	"strings"
	"testing"
)

func TestPlanFromDefectReport(t *testing.T) {
	// CIE 가 실제로 낸 답이다.
	analysis := "`cms` 에서 이 일에 관련된 파일을 하나씩 봤습니다.\n" +
		"**`src/lib/components/workplace/CalendarView.svelte`**\n" +
		"원인: exportEndDateStr = moment(currentDate).endOf('month')\n" +
		"이유: endOf 가 반환하는 값이 다를 수 있다\n" +
		"**`src/lib/server/workplace/workplace.ts`**\n" +
		"원인: lt: new Date(new Date().getFullYear(), new Date().getMonth() + 1, 0)\n" +
		"이유: lt 가 말일 00:00 이라 말일 데이터가 생략된다\n" +
		"**`src/lib/types/notifications.ts`**\n" +
		"[무관] 알림 타입 정의\n"

	got := PlanFromDefectReport(analysis)
	if len(got) != 2 {
		t.Fatalf("변경 %d개, want 2: %+v", len(got), got)
	}
	if got[1].FilePath != "src/lib/server/workplace/workplace.ts" {
		t.Errorf("경로 = %q", got[1].FilePath)
	}
	if !strings.Contains(got[1].Instructions, "getMonth() + 1, 0") {
		t.Errorf("문제의 줄이 지시문에 없다: %q", got[1].Instructions)
	}
	if !strings.Contains(got[1].Instructions, "말일 00:00") {
		t.Errorf("이유가 지시문에 없다: %q", got[1].Instructions)
	}
}

func TestPlanFromDefectReportIgnoresPlainSummaries(t *testing.T) {
	// 설명형 답에는 "원인:" 이 없다. 그때는 평소대로 모델에게 맡겨야 한다.
	analysis := "**`src/lib/server/db/workplace.ts`**\nworkplace 테이블과 관련된 함수들을 제공한다.\n"
	if got := PlanFromDefectReport(analysis); got != nil {
		t.Errorf("설명형 답에서 계획을 만들었다: %+v", got)
	}
	if got := PlanFromDefectReport("아무 형식도 아닌 글"); got != nil {
		t.Errorf("형식이 아닌데 계획을 만들었다: %+v", got)
	}
}
