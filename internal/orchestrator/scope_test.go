package orchestrator

import "testing"

func TestPathExistsInRepo(t *testing.T) {
	files := []string{
		"src/lib/server/db/workplace.ts",
		"src/lib/components/workplace/CalendarView.svelte",
		"src/routes/workplaces/+page.svelte",
	}
	yes := []string{
		"src/lib/server/db", "src/lib/server/db/workplace.ts",
		"/src/routes/workplaces/", "SRC/LIB/COMPONENTS",
	}
	no := []string{
		// 추출기가 경로 자리에 넣는 우리말 설명. 이게 통과하면 탐색이 빈다.
		"월별 근무 조회", "급여 정산 로직", "src/lib/server/dbx", "",
	}
	for _, p := range yes {
		if !pathExistsInRepo(p, files) {
			t.Errorf("있는 경로로 봐야 한다: %q", p)
		}
	}
	for _, p := range no {
		if pathExistsInRepo(p, files) {
			t.Errorf("없는 경로다: %q", p)
		}
	}
}
