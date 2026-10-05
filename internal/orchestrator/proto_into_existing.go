package orchestrator

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/guard"
)

var reMsgOpenLine = regexp.MustCompile(`^\s*message\s+(\w+)\s*\{`)

// fieldWentIntoNewMessage 는 담을 자리를 **새로 만든 메시지**에 넣은 것을 잡는다.
//
// 실측 W-53071 이다. 자리도 이름도 타입도 다 맞췄는데, 있는 `ReceivedRequest`
// 가 아니라 새로 만든 `message RequestConnect` 에 넣었다.
//
// \t+message RequestConnect {
// \t+  string request_id = 1;
// \t+  ConnectRequestStatus status = 2;
// \t+}
//
// 그 메시지를 받거나 돌려주는 RPC 가 없다 — **아무도 안 쓴다.** 빌드는
// 통과하고 다른 관문도 다 지난다. 그러면 상태가 담길 자리가 없는 채로
// 「다 했다」 가 된다.
//
// 넘길 때 어느 메시지인지 세어서 알려 주므로(StateType) 그것으로 확인한다.
func fieldWentIntoNewMessage(diff, stateType string) []guard.Violation {
	if strings.TrimSpace(stateType) == "" {
		return nil
	}
	var cur string // 지금 보고 있는, **새로 만든** 메시지
	hitExisting := false
	var intoNew []string

	for _, line := range strings.Split(diff, "\n") {
		if !strings.HasPrefix(line, "+") || strings.HasPrefix(line, "+++") {
			cur = ""
			continue
		}
		body := line[1:]
		if m := reMsgOpenLine.FindStringSubmatch(body); m != nil {
			cur = m[1]
			continue
		}
		if cur != "" && strings.Contains(body, "}") {
			cur = ""
			continue
		}
		if _, _, ok := protoField(body); !ok {
			continue
		}
		if cur == "" {
			hitExisting = true // 새로 연 메시지 밖 = 있던 메시지 안
		} else {
			intoNew = append(intoNew, cur)
		}
	}

	if hitExisting || len(intoNew) == 0 {
		return nil
	}
	return []guard.Violation{{
		Why: fmt.Sprintf("새로 만든 메시지(%s)에만 필드를 넣었다 — 있던 %s 에는 아무것도 안 들어갔다",
			strings.Join(uniqueStrings(intoNew), ", "), stateType),
		Evidence: []string{
			"새 메시지는 아무도 받거나 돌려주지 않는다. 상태가 담길 자리가 없는 것이다.",
			stateType + " 는 이미 있다. **그 안에** 필드를 더해라 — 같은 이름으로 새로 만들지 마라.",
		},
	}}
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
