package orchestrator

import (
	"fmt"
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
)

// stopIfStateMissing 은 담을 자리가 없으면 멈추고 그 까닭을 댄다.
//
// **건너뛸 때도 적는다.** 조용히 아무것도 안 하는 단계는 죽어 있어도 아무도
// 모른다 — 실제로 한 번 그렇게 지나갔다(W-24436).
func (t *taskContext) stopIfStateMissing() (bool, error) {
	plan, ok := t.ctx.Value("current_plan").(agent.Plan)
	if !ok || len(plan.Changes) == 0 {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "STATE_CHECK_SKIPPED",
			"계획이 파일을 알려 주지 않아 묻지 못했다", "", "")
		return false, nil
	}
	var files []string
	for _, c := range plan.Changes {
		files = append(files, c.FilePath)
	}

	ans, asked := t.askStateExists(files)
	if !asked {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "STATE_CHECK_SKIPPED",
			"쓸 수 있는 이름을 캐지 못해 묻지 못했다", "", strings.Join(clipList(files), " · "))
		return false, nil
	}
	if len(ans.missing) == 0 {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "STATE_FOUND",
			"담을 자리가 있다 — 이어서 만든다", "", ans.have)
		return false, nil
	}

	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "STATE_MISSING",
		"이 일에 필요한 상태를 담을 자리가 이 저장소에 없다", "", strings.Join(ans.missing, "\n"))
	if chain := t.chainForMissingState(ans.missing); len(chain) > 0 {
		t.pendingChain = chain
	}
	return true, fmt.Errorf("이 저장소만으로는 만들 수 없다 — 담을 자리가 없다:\n  %s",
		strings.Join(ans.missing, "\n  "))
}
