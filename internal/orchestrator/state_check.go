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

// askStateExists 는 이 일에 필요한 상태를 담을 자리가 있는지 묻는다.
// 못 물어봤으면 (nil, false) — 그때는 하던 대로 간다.
func (t *taskContext) askStateExists(files []string) (*stateAnswer, bool) {
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
