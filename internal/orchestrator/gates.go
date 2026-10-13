package orchestrator

import "github.com/connectfit-team/auto-coder-swarm/internal/agent"

// gateResult 는 넘기기 전 관문 하나가 막은 까닭이다.
type gateResult struct {
	label    string // 기록에 남길 이름
	reason   string // 사람이 읽을 한 줄
	note     string
	feedback string // 다음 시도에 줄 말
	revert   bool   // 작업 트리를 되돌린다
}

// gateChecks 는 사람에게 넘기기 전에 보는 관문을 차례대로 돌린다.
//
// handOver 와 모델이 부르는 check 도구가 **같은 함수**를 쓴다. 두 곳에서 따로
// 판정하면 도구가 통과라고 한 것을 관문이 막고, 모델은 고칠 길이 없다.
func (t *taskContext) gateChecks(diff string) []gateResult {
	var out []gateResult

	// 계획이 짚은 자리를 하나도 안 고쳤으면 한 일이 없는 것이다.
	if plan, ok := t.ctx.Value("current_plan").(agent.Plan); ok {
		if bad := CheckDidTheWork(plan, diff); len(bad) > 0 {
			note := AlignmentNote(bad)
			out = append(out, gateResult{
				label:  "NOT_THE_WORK",
				reason: "시킨 일을 한 흔적이 없다 — 넘기지 않는다",
				note:   note,
				feedback: "ALIGNMENT: " + note +
					"\n계획이 짚은 파일을 고쳐라. 상관없는 정리만 남기지 마라.",
				revert: true,
			})
		}
	}

	// 새로 만드는 일인데 새로 생긴 이름이 하나도 없으면 만든 것이 아니다.
	//
	// **요청문으로 판단한다.** t.newFeature 는 눈이 "못 찾았다" 고 했을 때만
	// 켜진다 — 눈이 무언가를 찾으면 꺼진 채로 지나가, 「기능을 추가할거야」 라는
	// 요청에 시험 파일 문자열 한 줄이 승인 대기까지 갔다(W-74462).
	if t.newFeature || IsNewFeatureRequest(t.req.UserRequest) {
		if bad := CheckNewFeatureAddedSomething(diff); len(bad) > 0 {
			note := AlignmentNote(bad)
			out = append(out, gateResult{
				label:  "NOTHING_NEW",
				reason: "새로 만들라고 했는데 새로 생긴 이름이 없다 — 넘기지 않는다",
				note:   note,
				feedback: "ALIGNMENT: " + note +
					"\n있던 코드를 감싸지 말고, 요청한 기능의 이름(함수·타입·열거 값)을 실제로 더해라.",
				revert: true,
			})
		}
	}

	// 담을 자리를 만드는 일인데 바꾸는 길이 없으면 반쪽이다.
	if t.req.AddsState {
		if bad := stateChangeNeedsAnRPC(t.repoPath, diff); len(bad) > 0 {
			note := AlignmentNote(bad)
			out = append(out, gateResult{
				label:  "STATE_HAS_NO_WAY_TO_CHANGE",
				reason: "담을 자리만 만들고 바꾸는 길을 안 냈다",
				note:   note,
				feedback: "PROTO: " + note +
					"\n\n고친 것은 그대로 두었다. **rpc 만 더해라** — 처음부터 다시 쓰지 마라.",
			})
		}
	}

	if bad := fieldWentIntoNewMessage(diff, t.req.StateType); len(bad) > 0 {
		note := AlignmentNote(bad)
		out = append(out, gateResult{
			label:  "STATE_HAS_NO_HOME",
			reason: "담을 자리를 새 메시지에 만들었다 — 아무도 안 쓴다",
			note:   note,
			feedback: "PROTO: " + note +
				targetMessageBody(t.repoPath, t.req.StateType) +
				"\n\n고친 것은 그대로 두었다. **위에 적힌 자리만 고쳐라** — 처음부터 다시 쓰지 마라.",
		})
	}

	// 계약을 고쳤으면 계약을 읽어 본다. 빌드는 계약을 못 본다 — 생성물을 다시
	// 만들지 않으므로 이미 있는 메시지를 다시 정의해도 go build 는 통과한다.
	//
	// **고친 것을 버리지 않는다.** 계약 수정은 대개 대부분 맞고 한두 가지가
	// 틀리다. 통째로 되돌리면 맞게 한 것까지 사라져 다음 시도가 처음부터 간다.
	// 되돌리지 않으면 작업 트리가 지저분해 restoreBest 가 건너뛰므로, 다음
	// 시도는 저절로 이 위에서 이어진다.
	if bad := CheckProtoChange(t.repoPath, diff); len(bad) > 0 {
		note := AlignmentNote(bad)
		out = append(out, gateResult{
			label:  "PROTO_BROKEN",
			reason: "계약이 깨진다 — 고친 것은 두고 그 자리만 다시 시킨다",
			note:   note,
			feedback: "PROTO: " + note +
				"\n\n고친 것은 그대로 두었다. **위에 적힌 자리만 고쳐라** — 처음부터 다시 쓰지 마라.",
		})
	}

	if bad := CheckToolLeak(diff); len(bad) > 0 {
		note := AlignmentNote(bad)
		out = append(out, gateResult{
			label:  "ALIGNMENT_BLOCKED",
			reason: "이 기계의 환경이 제품 코드에 새어 들어갔다 — 넘기지 않는다",
			note:   note,
			feedback: "ALIGNMENT: " + note +
				"\n이 기계의 주소·포트는 제품 코드에 넣지 마라. 요청한 것만 고쳐라.",
			revert: true,
		})
	}

	return out
}
