package orchestrator

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
)

// 코드를 쓰기 전에 **만들 수 있는 일인지** 먼저 묻는다.
//
// 새 기능은 대개 **새 상태**를 필요로 한다. 그 상태를 담을 자리가 없으면
// 프런트만으로는 만들 수 없다. 그런데 물어본 적이 없어서, 모델은 늘 있는
// 필드를 억지로 갖다 썼다.
//
//	const pendingApproval = staffs.filter((s) => s.invited === false);
//
// 「아직 초대하지 않음」 을 「보류」 로 쓴 것이다. 코드는 썼지만 **무엇을
// 만들지 정의하지 못했다.** 사람이라면 여기서 "이건 뒤쪽에 상태가 먼저
// 있어야 한다" 고 말하고 멈춘다.
//
// 한 번에 하나만 묻는다 — 「이 일에 필요한 상태를 담을 자리가 여기 있나?」
// 답은 이름 하나 아니면 NONE 이다.

const stateCheckTimeout = 3 * time.Minute

type stateAnswer struct {
	have    string   // 담을 자리가 있으면 그 이름
	missing []string // 없으면 있어야 할 이름들
}

var reNone = regexp.MustCompile(`(?i)^\s*NONE\b`)

// askStateExists 는 **세 번 물어 다수결로 정한다.**
//
// 한 번만 물었더니 회차마다 답이 달랐다 — 같은 요청에 어떤 회차는 「없다」
// 고 바르게 답하고(W-30084 외) 어떤 회차는 「있다」 로 넘어가 코드를 썼다
// (W-90966). 이 판단이 흔들리면 그 뒤가 전부 흔들린다.
//
// 값을 지어내는 것이 아니라 **고르는** 물음이므로, 여러 번 묻고 모아 보면
// 흔들림이 줄어든다. 「없다」 가 과반이면 없는 것으로 본다 — 없는데 있다고
// 해서 반쪽을 만드는 쪽이, 있는데 없다고 해서 멈추는 쪽보다 나쁘다.
func (t *taskContext) askStateExists(files []string) (*stateAnswer, bool) {
	const rounds = 3
	var answers []*stateAnswer
	for i := 0; i < rounds; i++ {
		if a, ok := t.askStateOnce(files); ok {
			answers = append(answers, a)
		}
	}
	if len(answers) == 0 {
		return nil, false
	}
	missingVotes := 0
	var missing []string
	var have string
	for _, a := range answers {
		if len(a.missing) > 0 {
			missingVotes++
			if len(a.missing) > len(missing) {
				missing = a.missing
			}
		} else if have == "" {
			have = a.have
		}
	}
	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "STATE_VOTE",
		fmt.Sprintf("담을 자리가 없다 %d표 / 물어본 %d번", missingVotes, len(answers)),
		"", strings.Join(missing, "\n"))

	if missingVotes*2 > len(answers) {
		return &stateAnswer{missing: missing}, true
	}
	return &stateAnswer{have: have}, true
}

// askStateOnce 는 한 번 묻는다.
func (t *taskContext) askStateOnce(files []string) (*stateAnswer, bool) {
	sheet := t.stateSheet(files)
	if strings.TrimSpace(sheet) == "" {
		return nil, false
	}

	prompt := fmt.Sprintf(`아래는 %s 저장소에서 이 일과 맞닿은 자리에 **실제로 있는 이름과 필드**다.
파일을 읽어 센 것이라 여기 없는 것은 없는 것이다.

%s
[요청]
%s

이 요청을 이루려면 **어떤 상태를 저장해야 하나?** 그리고 그 상태를 담을
자리가 위에 있나?

  있으면   그 필드 이름 하나만 적어라.        보기: ConnectableStaff.invited
  없으면   NONE 이라 적고, 줄을 바꿔 **있어야 할 이름**을 적어라.
           보기:
           NONE
           연결 요청에 보류 상태를 담을 필드

다른 말은 쓰지 마라.`, t.targetRepo, sheet, strings.TrimSpace(t.req.UserRequest))

	ctx, cancel := context.WithTimeout(t.ctx, stateCheckTimeout)
	defer cancel()
	raw, err := agent.CallLLM(ctx, t.primaryLLM, "StateCheck", prompt)
	if err != nil || strings.TrimSpace(raw) == "" {
		return nil, false
	}

	lines := strings.Split(strings.TrimSpace(raw), "\n")
	first := strings.TrimSpace(lines[0])
	if !reNone.MatchString(first) {
		if first == "" {
			return nil, false
		}
		return &stateAnswer{have: first}, true
	}
	var missing []string
	for _, l := range lines[1:] {
		if l = strings.TrimSpace(l); l != "" {
			missing = append(missing, l)
		}
	}
	return &stateAnswer{missing: missing}, true
}

// stateSheet 는 그 자리들의 이름·필드를 모은다. 쓰기 전에 주는 쪽지와 같은 것이다.
func (t *taskContext) stateSheet(files []string) string {
	var b strings.Builder
	n := 0
	for _, f := range files {
		if n >= 3 {
			break
		}
		if s := AvailableNames(t.repoPath, f); s != "" {
			b.WriteString(s)
			n++
		}
	}
	return b.String()
}
