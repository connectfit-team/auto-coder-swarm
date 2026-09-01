package orchestrator

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
)

func (t *taskContext) stepExecution(attempt int) error {
	plan := t.ctx.Value("current_plan").(agent.Plan)
	var failures []string
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
		// **못 고쳤으면 못 고쳤다고 남긴다.**
		//
		// 그동안 이 오류를 버렸다. 그래서 파일이 안 바뀐 채로 빌드로 넘어가고,
		// 실패가 "빌드 오류" 로 둔갑했다. 무엇이 왜 안 됐는지가 다음 계획의
		// 되먹임이 된다.
		if _, err := t.coder.ModifyFile(t.ctx, filepath.Join(t.repoPath, change.FilePath), change.Instructions); err != nil {
			t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CODING_FAILED",
				fmt.Sprintf("[%s] 고치지 못했습니다", change.FilePath), "", err.Error())
			failures = append(failures, err.Error())
		}
	}
	if len(failures) > 0 {
		t.lastFeedback = "CODER FAILED:\n" + strings.Join(failures, "\n")
	}
	return nil
}
