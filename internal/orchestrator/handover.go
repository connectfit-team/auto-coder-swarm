package orchestrator

import (
	"os/exec"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
)

// 사람에게 넘기는 길은 하나여야 한다.
//
// 결함 흐름에는 승인 대기로 가는 길이 셋 있었다 — 검토자가 반대했지만
// 분석이 짚은 수정, 검토를 통과한 수정, 그리고 최대 시도를 다 쓰고도 고친
// 것이 남은 경우. 「시킨 일인가」 를 보는 관문은 그 가운데 한 곳에만 달려
// 있었고, W-76095 는 관문이 없는 세 번째 길로 나갔다. 연결보류를 만들라는
// 요청에 이 기계의 vLLM 주소를 넣은 코드가 승인 대기까지 갔다.
//
// 문을 하나 더 다는 것으로는 못 막는다. 길을 하나로 만든다 —
// 새 길을 내면 TestOnlyOneDoorToApproval 이 막는다.

// handOver 는 고친 것을 사람 판단으로 넘긴다.
// 넘길 수 없으면 false 다 — 부르는 쪽이 되먹임을 들고 다시 시도한다.
func (t *taskContext) handOver(diff, why string) (RunResult, bool) {
	// 계획이 짚은 자리를 하나도 안 고쳤으면 한 일이 없는 것이다.
	if plan, ok := t.ctx.Value("current_plan").(agent.Plan); ok {
		if bad := CheckDidTheWork(plan, diff); len(bad) > 0 {
			note := AlignmentNote(bad)
			t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "NOT_THE_WORK",
				"시킨 일을 한 흔적이 없다 — 넘기지 않는다", why, note)
			t.lastFeedback = "ALIGNMENT: " + note +
				"\n계획이 짚은 파일을 고쳐라. 상관없는 정리만 남기지 마라."
			exec.CommandContext(t.ctx, "git", "-C", t.repoPath, "checkout", ".").Run()
			return RunResult{}, false
		}
	}

	if bad := CheckToolLeak(diff); len(bad) > 0 {
		note := AlignmentNote(bad)
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "ALIGNMENT_BLOCKED",
			"이 기계의 환경이 제품 코드에 새어 들어갔다 — 넘기지 않는다", why, note)
		t.lastFeedback = "ALIGNMENT: " + note +
			"\n이 기계의 주소·포트는 제품 코드에 넣지 마라. 요청한 것만 고쳐라."
		exec.CommandContext(t.ctx, "git", "-C", t.repoPath, "checkout", ".").Run()
		return RunResult{}, false
	}
	if diff != "" {
		t.orchestrator.store.UpdateTaskProposedDiff(t.taskID, diff)
	}
	return RunResult{RepoName: t.targetRepo, WaitingApproval: true}, true
}
