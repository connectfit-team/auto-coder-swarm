package orchestrator

import (
	"os/exec"
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
	// **빌드를 통과하지 못한 수정은 넘기지 않는다.**
	//
	// 사람이 승인하면 그대로 밀린다. 빌드가 깨지는 것을 승인 대기에 올려
	// 두는 것은 사람의 시간을 쓰는 것이 아니라 버리는 것이다.
	//
	// 실측으로 타입 오류 3개가 남은 수정이 승인 대기까지 갔다(W-86009).
	// 검토자가 반대해도 넘기는 길이 있는데(그건 맞다 — 검토자가 틀릴 수
	// 있다), 그 길이 빌드까지 못 본 것을 함께 흘려보냈다.
	if !t.verifiedClean {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "NOT_VERIFIED",
			"빌드를 통과하지 못한 수정이라 넘기지 않는다", why, "")
		return RunResult{}, false
	}

	// 관문은 gateChecks 한 곳에 있다 — 모델이 부르는 check 도구와 같은 것을 본다.
	for _, g := range t.gateChecks(diff) {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, g.label, g.reason, why, g.note)
		t.lastFeedback = g.feedback
		if g.revert {
			exec.CommandContext(t.ctx, "git", "-C", t.repoPath, "checkout", ".").Run()
		}
		return RunResult{}, false
	}
	if diff != "" {
		t.orchestrator.store.UpdateTaskProposedDiff(t.taskID, diff)
	}

	// **관문을 다 지났으면 스스로 PR 까지 간다 — 켜 둔 저장소에서만.**
	//
	// 오픈소스 코딩 에이전트는 고치고 검사하고 내놓는 것까지 한 흐름이다.
	// 여기서 멈추면 사람이 누르기 전에는 아무 일도 일어나지 않는다.
	// 머지는 하지 않는다 — 초안으로 열고 사람이 본다.
	if url, ok := t.openPRMyself(why); ok {
		return RunResult{RepoName: t.targetRepo, Result: url}, true
	}

	// 계약 저장소는 승인 뒤에도 밀어서 펴내는 것이 아니다. 사람이 볼 화면에
	// 다음 걸음을 적어 둔다 — 비워 두면 여기서 일이 멈춘다.
	return RunResult{
		RepoName:        t.targetRepo,
		Result:          howToPublishRepo(t.targetRepo),
		WaitingApproval: true,
	}, true
}
