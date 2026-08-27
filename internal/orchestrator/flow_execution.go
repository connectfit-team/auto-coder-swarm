package orchestrator

import (
	"fmt"
	"path/filepath"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
)

func (t *taskContext) stepExecution(attempt int) error {
	plan := t.ctx.Value("current_plan").(agent.Plan)
	for _, change := range plan.Changes {
		// 생성물은 고치기 전에 막는다. 고쳐도 다음 생성 때 덮어써져 조용히
		// 사라지고, 그 사이 소비하는 서비스만 깨진다.
		if isGeneratedPath(change.FilePath) {
			t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "SKILL_BLOCK",
				fmt.Sprintf("[%s] 생성물이라 수정하지 않음", change.FilePath), "", "")
			t.lastFeedback = fmt.Sprintf(
				"생성물 %s 를 대상으로 잡았다. 생성물은 손으로 고치지 않는다 — 원본(.proto 등)을 고치고 발행 절차를 따르라.",
				change.FilePath)
			continue
		}
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CODING", fmt.Sprintf("[%s] 수정", change.FilePath), change.Instructions, "")
		t.coder.ModifyFile(t.ctx, filepath.Join(t.repoPath, change.FilePath), change.Instructions)
	}
	return nil
}
