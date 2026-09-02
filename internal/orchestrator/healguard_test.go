package orchestrator

import (
	"testing"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
)

func TestAllowedToHeal(t *testing.T) {
	plan := agent.Plan{Changes: []agent.FileChange{
		{FilePath: "src/lib/server/workplace/workplace.ts"},
	}}
	buildErr := `RollupError: "getAttendanceRecords" is not exported by "src/lib/server/db/attendance.ts", imported by "src/routes/api/attendance/records/+server.ts".`

	if !allowedToHeal("src/lib/server/workplace/workplace.ts", plan, buildErr) {
		t.Error("계획한 파일을 막았다")
	}
	if !allowedToHeal("./src/lib/server/workplace/workplace.ts", plan, buildErr) {
		t.Error("./ 가 붙은 같은 경로를 다른 파일로 봤다")
	}
	if !allowedToHeal("src/lib/server/db/attendance.ts", plan, buildErr) {
		t.Error("오류가 이름을 댄 파일을 막았다")
	}
	// 요청하지도, 오류가 지목하지도 않은 파일. 실제로 이런 파일이 diff 에
	// 섞여 승인 대기까지 갔다.
	if allowedToHeal("src/routes/api/workplaces/[id]/work-stamps/export/+server.ts", plan, buildErr) {
		t.Error("상관없는 파일을 건드리게 뒀다")
	}
	if allowedToHeal("", plan, buildErr) {
		t.Error("빈 경로를 통과시켰다")
	}
}

func TestSameFilePath(t *testing.T) {
	if !sameFilePath("./a/b.ts", "a/b.ts") || !sameFilePath("/a/b.ts", "a/b.ts") {
		t.Error("같은 경로를 다르게 봤다")
	}
	if sameFilePath("a/b.ts", "a/c.ts") {
		t.Error("다른 경로를 같게 봤다")
	}
}
