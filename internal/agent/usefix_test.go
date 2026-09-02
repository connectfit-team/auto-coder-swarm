package agent

import (
	"strings"
	"testing"
)

func TestPlanFromDefectReportCarriesFix(t *testing.T) {
	analysis := "**`src/lib/server/workplace/workplace.ts`**\n" +
		"원인: lt: new Date(new Date().getFullYear(), new Date().getMonth() + 1, 0)\n" +
		"이유: 말일 00:00 이 경계라 그 날이 통째로 빠진다\n" +
		"고침: lt: new Date(new Date().getFullYear(), new Date().getMonth() + 1, 1)\n" +
		"확신: 높음\n"
	got := PlanFromDefectReport(analysis)
	if len(got) != 1 {
		t.Fatalf("변경 %d개", len(got))
	}
	if !strings.Contains(got[0].Instructions, "이렇게 바꿔라") {
		t.Errorf("고칠 값을 안 전했다: %q", got[0].Instructions)
	}
	if !strings.Contains(got[0].Instructions, "+ 1, 1)") {
		t.Errorf("바뀐 줄이 없다: %q", got[0].Instructions)
	}
}

func TestPlanFromDefectReportWithoutFixLine(t *testing.T) {
	// 고침 줄이 없어도 예전처럼 원인·이유만으로 돈다.
	analysis := "**`a.ts`**\n원인: x\n이유: y\n"
	got := PlanFromDefectReport(analysis)
	if len(got) != 1 || strings.Contains(got[0].Instructions, "이렇게 바꿔라") {
		t.Errorf("고침 없는 경우가 깨졌다: %+v", got)
	}
}
