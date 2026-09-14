package orchestrator

import (
	"fmt"
	"os"
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
		// 새로 만드는 일이면 코더에게도 그것을 알린다. 계획에만 붙이면
		// 코더는 여전히 "고치기 전" 코드를 지어낸다(실측 W-34685).
		instr := change.Instructions
		if t.newFeature {
			instr = CoderNewFeatureHint() + instr
		}
		// **쓰기 전에 있는 이름을 보여 준다.**
		//
		// 사람은 이럴 때 grep 한 번 하고 나서 쓴다. 기계는 그 한 걸음을
		// 건너뛰고 그럴듯한 이름을 지어냈다 — getConnectClient().updateInviteStatus
		// 같은 것(W-19079). 이 파일이 들여오는 저장소 안 모듈의 내보낸 이름과,
		// 그 공장에 저장소가 실제로 쓰는 메서드를 파일에서 세어 먼저 준다.
		if sheet := AvailableNames(t.repoPath, change.FilePath); sheet != "" {
			instr = sheet + instr
		}
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CODING", fmt.Sprintf("[%s] 수정", change.FilePath), instr, "")
		// **못 고쳤으면 못 고쳤다고 남긴다.**
		//
		// 그동안 이 오류를 버렸다. 그래서 파일이 안 바뀐 채로 빌드로 넘어가고,
		// 실패가 "빌드 오류" 로 둔갑했다. 무엇이 왜 안 됐는지가 다음 계획의
		// 되먹임이 된다.
		full := filepath.Join(t.repoPath, change.FilePath)

		// **없는 파일이면 새로 만든다.**
		//
		// 계획은 이미 새 파일을 허용한다(폴더가 있으면 살린다). 그런데 손은
		// 읽기부터 해서 첫 줄에서 죽었다 — 없는 기능을 만들라는 요청에
		// 새 파일을 못 만드는 것은 앞뒤가 안 맞는다(W-44018).
		if _, statErr := os.Stat(full); statErr != nil {
			if _, err := t.coder.CreateFile(t.ctx, full, instr, ""); err != nil {
				t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CODING_FAILED",
					fmt.Sprintf("[%s] 새로 만들지 못했습니다", change.FilePath), "", err.Error())
				failures = append(failures, err.Error())
			} else {
				t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CODING_CREATED",
					fmt.Sprintf("[%s] 새로 만들었습니다", change.FilePath), "", "")
			}
			continue
		}

		if _, err := t.coder.ModifyFile(t.ctx, full, instr); err != nil {
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

	// **마지막 시도에서는 고친 것을 버리지 않는다.**
	//
	// 못 고친 파일이 있으면 다시 계획하는 것이 맞다(반쪽 빌드는 까닭을
	// 가린다). 그런데 시도를 다 쓰고도 못 고친 것이 남으면, 고친 것까지
	// 함께 버려진다 — 실측으로 넷 가운데 셋을 고쳐 놓고 하나 때문에
	// 통째로 실패했다(W-54920).
	//
	// 뭉개진 것은 이미 저장되지 않는다(문법 관문). 그러니 마지막에는
	// 빌드에게 판정을 맡긴다. 빌드가 통과하면 그 변경은 성립한 것이고,
	// 안 되면 진짜 오류가 나온다 — 둘 다 지금보다 낫다.
	if len(failures) < len(plan.Changes) {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CODING_PARTIAL_KEPT",
			fmt.Sprintf("%d개 가운데 %d개를 고쳤다 — 마지막 시도라 빌드에 맡긴다",
				len(plan.Changes), len(plan.Changes)-len(failures)), "", strings.Join(failures, "\n"))
		return nil
	}
	return fmt.Errorf("계획한 파일 %d개를 하나도 고치지 못했다", len(plan.Changes))
}
