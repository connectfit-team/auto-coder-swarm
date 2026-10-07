package orchestrator

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/guard"
)

var (
	reMsgOpen  = regexp.MustCompile(`^\s*message\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{`)
	reProtoFld = regexp.MustCompile(`^\s*(?:repeated\s+|optional\s+|required\s+)?[A-Za-z_][\w.]*\s+[A-Za-z_]\w*\s*=\s*\d+\s*;`)
)

// emptyNewMessage 는 **속이 빈 채로 새로 만든 메시지**를 잡는다.
//
// 실측 W-13896 이다. 자리도 이름도 맞게 갔는데 속이 없었다.
//
//	rpc UpdateReceivedRequest(RequestUpdateReceivedRequest) returns (ResponseUpdateReceivedRequest) {}
//
//	message RequestUpdateReceivedRequest {
//	  // 필요한 필드 추가
//	}
//	message ResponseUpdateReceivedRequest {
//	  // 필요한 필드 추가
//	}
//
// 빌드는 통과한다 — proto 에서 빈 메시지는 문법에 맞다. 그래서 여기서 안
// 잡으면 **쓸 수 없는 계약이 관문을 다 지난다.** 무엇을 담아 보내고 무엇을
// 돌려받는지가 없으면 그 RPC 는 부를 수가 없다.
//
// 일부러 비우는 메시지는 있다 — `message RequestListSentInvites {}` 처럼 한
// 줄로 닫은 것이다. 그것은 「받을 것이 없다」 는 뜻이라 두고, **여러 줄로
// 열어 놓고 속이 주석뿐인 것**만 잡는다.
func emptyNewMessage(repoPath, diff string) []guard.Violation {
	var out []guard.Violation
	var cur string
	var hasField, multiline bool

	closeCur := func() {
		if cur != "" && multiline && !hasField {
			out = append(out, guard.Violation{
				Why:      fmt.Sprintf("message %s 를 속이 빈 채로 만들었다 — 필드가 하나도 없다", cur),
				Evidence: evidenceForEmpty(repoPath),
			})
		}
		cur, hasField, multiline = "", false, false
	}

	for _, line := range strings.Split(diff, "\n") {
		if !strings.HasPrefix(line, "+") || strings.HasPrefix(line, "+++") {
			closeCur()
			continue
		}
		body := line[1:]
		if cur == "" {
			if m := reMsgOpen.FindStringSubmatch(body); m != nil {
				cur = m[1]
				// 한 줄로 열고 닫은 것은 「받을 것이 없다」 는 뜻이다.
				multiline = !strings.Contains(body[strings.Index(body, "{"):], "}")
				if !multiline {
					cur = ""
				}
			}
			continue
		}
		if strings.Contains(body, "}") {
			closeCur()
			continue
		}
		if reProtoFld.MatchString(body) {
			hasField = true
		}
	}
	closeCur()
	return out
}

// evidenceForEmpty 는 「본떠라」 라고 말하는 대신 **본뜰 것을 보여 준다.**
//
// 실측 W-14983 이 `ResponseUpdateReceivedRequestHoldStatus` 를 빈 채로 냈다.
// 쪽지에는 "이 계약에 이미 있는 짝을 본떠 필드를 채워라" 고만 적혀 있었다.
// 이 세션 내내 같은 교훈이었다 — 말로 이르는 것과 실물을 한 번 보여 주는 것은
// 다르다(#151·#152·#170·#179·#184).
func evidenceForEmpty(repoPath string) []string {
	ev := []string{
		"무엇을 담아 보내고 무엇을 돌려받는지가 없으면 그 RPC 는 부를 수 없다.",
		"이 계약에 이미 있는 짝을 본떠 필드를 채워라 — 「필요한 필드 추가」 같은 자리표시는 두지 마라.",
	}
	if pair := aPairToCopy(repoPath); pair != "" {
		ev = append(ev, "[이 계약에 이미 있는 짝 — 이렇게 채운다]\n"+pair)
	}
	return ev
}

// aPairToCopy 는 이 계약에 있는 짧은 요청·응답 짝 하나를 준다.
func aPairToCopy(repoPath string) string {
	if repoPath == "" {
		return ""
	}
	var found string
	forEachProto(repoPath, func(rel, _ string) {
		if found != "" {
			return
		}
		if pairs := requestPairsIn(protoBlocks(protoAtHead(repoPath, rel)), 1); len(pairs) > 0 {
			found = pairs[0]
		}
	})
	return found
}
