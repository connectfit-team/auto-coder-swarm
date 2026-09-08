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
	if len(failures) == 0 {
		return nil
	}

	// **반쪽만 고쳐진 채로 빌드로 넘어가지 않는다.**
	//
	// 못 고친 것을 되먹임으로 남기기는 했지만 그대로 빌드로 갔다. 그래서
	// 계획한 파일 넷 가운데 하나만 고쳐진 코드가 빌드에 들어가고, 자가치유가
	// 세 번을 태운 뒤 "최대 시도 초과" 로 끝났다(W-32501). 실패의 까닭이
	// 「코더가 세 파일을 못 고쳤다」 에서 「빌드가 안 된다」 로 둔갑한다.
	//
	// 되먹임을 들고 계획으로 돌아간다. 횟수는 이미 셋으로 묶여 있다 —
	// 새 고리를 만드는 것이 아니다.
	t.lastFeedback = fmt.Sprintf(
		"CODER FAILED: 계획한 파일 %d개 가운데 %d개를 고치지 못했다. 고칠 수 있는 파일만 골라 다시 계획하라.\n%s",
		len(plan.Changes), len(failures), strings.Join(failures, "\n"))
	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CODING_PARTIAL",
		fmt.Sprintf("파일 %d개 가운데 %d개를 못 고쳤다 — 반쪽 상태로 빌드하지 않는다",
			len(plan.Changes), len(failures)), "", strings.Join(failures, "\n"))
	if attempt < 3 {
		return errRetryPlanning
	}
	return fmt.Errorf("계획한 파일 %d개 가운데 %d개를 고치지 못했다", len(plan.Changes), len(failures))
}
