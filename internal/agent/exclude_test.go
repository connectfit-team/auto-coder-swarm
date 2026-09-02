package agent

import "testing"

const twoCandidates = "**`src/a.ts`**\n" +
	"원인: `lt: x`\n이유: 경계가 틀렸다\n확신: 높음\n" +
	"**`src/b.ts`**\n" +
	"원인: `lte: y`\n이유: 경계가 틀렸다\n확신: 높음\n"

func TestPlanFromDefectReportExcluding(t *testing.T) {
	all := PlanFromDefectReport(twoCandidates)
	if len(all) != 2 {
		t.Fatalf("후보 2개여야 하는데 %d개", len(all))
	}

	// 막다른 길로 드러난 파일은 빠진다.
	rest := PlanFromDefectReportExcluding(twoCandidates, []string{"src/a.ts"})
	if len(rest) != 1 || rest[0].FilePath != "src/b.ts" {
		t.Fatalf("다음 후보로 넘어가지 못했다: %+v", rest)
	}

	// 전부 막다른 길이면 비운다 — 그때는 모델 계획으로 넘어간다.
	none := PlanFromDefectReportExcluding(twoCandidates, []string{"src/a.ts", "src/b.ts"})
	if len(none) != 0 {
		t.Fatalf("후보가 남으면 안 된다: %+v", none)
	}

	// 확신 낮은 것만 남으면 그것을 쓴다.
	mixed := "**`src/a.ts`**\n원인: `lt: x`\n이유: r\n확신: 높음\n" +
		"**`src/c.ts`**\n원인: `gte: z`\n이유: r\n확신: 낮음\n"
	weak := PlanFromDefectReportExcluding(mixed, []string{"src/a.ts"})
	if len(weak) != 1 || weak[0].FilePath != "src/c.ts" {
		t.Fatalf("확신 낮은 후보로 못 넘어갔다: %+v", weak)
	}
}
