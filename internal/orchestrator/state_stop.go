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
		// **못 물었다고 그냥 지나가지 않는다 — 넓혀서 다시 묻는다.**
		//
		// 계획이 **아직 없는 파일**(새로 만들 시험 파일 등)을 짚으면 그 파일에서
		// 쓸 수 있는 이름을 못 캔다. 그러면 「담을 자리가 없다」 물음을 통째로
		// 건너뛰고, 연쇄가 안 일어나 빈 수정으로 끝난다 — 실측 W-40774 가
		// `tests/crud/connection.spec.ts` 하나로 그렇게 됐다(갈래 A 가 일곱 판
		// 만에 처음 떨어진 자리다).
		//
		// 분석이 찾아 준 후보는 이미 손에 있다. 모델을 더 부르지도, CIE 에 다시
		// 묻지도 않는다 — 들고 있는 것을 쓰는 것뿐이다.
		if wider := widenWithCandidates(files, t.candidatePaths); len(wider) > len(files) {
			ans, asked = t.askStateExists(wider)
			if asked {
				t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "STATE_CHECK_WIDENED",
					fmt.Sprintf("계획한 파일로는 못 캐서 후보 %d개로 넓혔다", len(wider)-len(files)),
					"", strings.Join(clipList(wider), " · "))
			}
		}
	}
	if !asked {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "STATE_CHECK_SKIPPED",
			"넓혀 봐도 쓸 수 있는 이름을 캐지 못해 묻지 못했다", "", strings.Join(clipList(files), " · "))
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

// widenWithCandidates 는 계획한 파일 뒤에 후보 파일을 덧붙인다.
//
// 계획한 것을 **앞에** 둔다 — 그쪽이 더 관련 있다. 뒤에 붙은 후보는 앞엣것이
// 아무것도 못 낼 때만 쓰이게 된다(stateSheet 이 앞에서부터 담는다).
func widenWithCandidates(planned, candidates []string) []string {
	seen := make(map[string]bool, len(planned))
	out := make([]string, 0, len(planned)+len(candidates))
	for _, f := range planned {
		if f == "" || seen[f] {
			continue
		}
		seen[f] = true
		out = append(out, f)
	}
	for _, f := range candidates {
		if f == "" || seen[f] {
			continue
		}
		seen[f] = true
		out = append(out, f)
	}
	return out
}
