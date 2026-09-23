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

	// 새로 만드는 일인데 새로 생긴 이름이 하나도 없으면 만든 것이 아니다.
	//
	// **요청문으로 판단한다.** 전에는 t.newFeature 만 봤는데, 그것은 눈이
	// "못 찾았다" 고 했을 때만 켜진다. 눈이 무언가를 찾았다고 하면 꺼진
	// 채로 지나가, 「기능을 추가할거야」 라는 요청에 시험 파일 문자열 한 줄이
	// 승인 대기까지 갔다(W-74462).
	if t.newFeature || IsNewFeatureRequest(t.req.UserRequest) {
		if bad := CheckNewFeatureAddedSomething(diff); len(bad) > 0 {
			note := AlignmentNote(bad)
			t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "NOTHING_NEW",
				"새로 만들라고 했는데 새로 생긴 이름이 없다 — 넘기지 않는다", why, note)
			t.lastFeedback = "ALIGNMENT: " + note +
				"\n있던 코드를 감싸지 말고, 요청한 기능의 이름(함수·타입·열거 값)을 실제로 더해라."
			exec.CommandContext(t.ctx, "git", "-C", t.repoPath, "checkout", ".").Run()
			return RunResult{}, false
		}
	}

	// 계약을 고쳤으면 계약을 읽어 본다. 빌드는 계약을 못 본다 —
	// 생성물을 다시 만들지 않으므로 이미 있는 메시지를 다시 정의해도
	// go build 는 통과한다.
	if bad := CheckProtoChange(t.repoPath, diff); len(bad) > 0 {
		note := AlignmentNote(bad)
		// **고친 것을 버리지 않는다.**
		//
		// 계약 수정은 대개 대부분 맞고 한두 가지가 틀리다 — 필드는 바르게
		// 더해 놓고 rpc 를 새 service 에 넣는 식이다. 통째로 되돌리면 맞게
		// 한 것까지 사라져 다음 시도가 처음부터 간다. aider 는 맞은 블록을
		// 써 두고 「나머지는 다시 보내지 마라」 고 일러 준다.
		//
		// 되돌리지 않으면 작업 트리가 지저분해 restoreBest 가 건너뛰므로,
		// 다음 시도는 저절로 이 위에서 이어진다.
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "PROTO_BROKEN",
			"계약이 깨진다 — 고친 것은 두고 그 자리만 다시 시킨다", why, note)
		t.lastFeedback = "PROTO: " + note +
			"\n\n고친 것은 그대로 두었다. **위에 적힌 자리만 고쳐라** — 처음부터 다시 쓰지 마라."
		return RunResult{}, false
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
