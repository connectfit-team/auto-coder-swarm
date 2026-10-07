package orchestrator

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	reMessageOpen = regexp.MustCompile(`^\s*message\s+(\w+)\s*\{`)
	reFieldNumber = regexp.MustCompile(`=\s*(\d+)\s*;`)
)

// targetMessageBody 는 필드를 넣을 **그 메시지의 진짜 몸통**을 보여 준다.
//
// 관문은 새 메시지에 넣은 것을 잡아내고 이름까지 댄다("ReceivedRequest 는
// 이미 있다. 그 안에 필드를 더해라"). 그런데 자식은 세 시도 내내 다시 새
// 메시지를 만들었다(실측 W-51583·W-40476, 둘 다 STATE_HAS_NO_HOME 로 끝났다).
// 이름만으로는 그 안에 무엇이 몇 번으로 들어 있는지 모르니 모양을 지어낸다 —
// #170·#151·#152 에서 겪은 것과 같다. 말로 이르는 것으로는 안 멈춘다.
//
// HEAD 에서 읽는다. 작업 트리에는 이번 시도가 만든 가짜 메시지가 들어 있어서,
// 그것을 보여 주면 자기가 만든 것을 본보기로 삼는다(#166).
func targetMessageBody(repoPath, message string) string {
	if repoPath == "" || message == "" {
		return ""
	}
	rel := declaringFileAtHead(repoPath, message)
	if rel == "" {
		return ""
	}
	block, next := messageBlock(protoAtHead(repoPath, rel), message)
	if block == "" {
		return ""
	}
	return fmt.Sprintf(
		"\n[필드를 넣을 자리] `%s` — %s 에 이미 있다.\n"+
			"```proto\n%s\n```\n"+
			"**이 메시지 안, 닫는 괄호 바로 앞에 한 줄을 더해라. 다음 빈 번호는 %d 이다.**\n"+
			"새 메시지를 만들지 마라 — 아무도 받거나 돌려주지 않아 상태가 담길 자리가 없다.\n\n",
		message, rel, block, next)
}

// targetMessageIn 은 그 메시지를 선언한 파일을 고칠 때만 몸통을 보여 준다.
func targetMessageIn(repoPath, message, rel string) string {
	if rel == "" || declaringFileAtHead(repoPath, message) != rel {
		return ""
	}
	return targetMessageBody(repoPath, message)
}

// declaringFileAtHead 는 그 메시지를 선언한 .proto 를 HEAD 기준으로 찾는다.
func declaringFileAtHead(repoPath, message string) string {
	if repoPath == "" || message == "" {
		return ""
	}
	want := regexp.MustCompile(`(?m)^\s*message\s+` + regexp.QuoteMeta(message) + `\s*\{`)
	found := ""
	forEachProto(repoPath, func(rel, _ string) {
		if found != "" {
			return
		}
		if want.MatchString(protoAtHead(repoPath, rel)) {
			found = rel
		}
	})
	return found
}

// messageBlock 은 그 메시지의 선언 덩어리와 **다음 빈 필드 번호**를 준다.
//
// 번호는 속 메시지의 것을 세지 않는다 — 깊이 1 의 필드만 본다. 속엣것까지
// 세면 바깥 메시지의 다음 번호가 엉뚱하게 커진다.
func messageBlock(src, name string) (string, int) {
	lines := strings.Split(src, "\n")
	start := -1
	for i, l := range lines {
		if m := reMessageOpen.FindStringSubmatch(l); m != nil && m[1] == name {
			start = i
			break
		}
	}
	if start < 0 {
		return "", 0
	}

	depth, end, max := 0, -1, 0
	for i := start; i < len(lines); i++ {
		l := lines[i]
		if depth == 1 {
			if m := reFieldNumber.FindStringSubmatch(l); m != nil {
				if n, err := strconv.Atoi(m[1]); err == nil && n > max {
					max = n
				}
			}
		}
		depth += strings.Count(l, "{") - strings.Count(l, "}")
		if depth <= 0 && strings.Contains(l, "}") {
			end = i
			break
		}
	}
	if end < 0 {
		return "", 0 // 닫히지 않았다 — 반쪽을 보여 주면 그 모양을 따라 쓴다
	}

	body := lines[start : end+1]
	// 긴 메시지는 머리와 꼬리만. 꼬리를 길게 두는 것은 거기에 큰 번호와
	// 닫는 괄호가 있어서다 — 어디에 넣을지는 꼬리를 봐야 안다.
	if len(body) > 40 {
		cut := len(body) - 26
		trimmed := append([]string{}, body[:6]...)
		trimmed = append(trimmed, fmt.Sprintf("  // … 가운데 %d 줄 줄임", cut))
		body = append(trimmed, body[len(body)-20:]...)
	}
	return strings.TrimRight(strings.Join(body, "\n"), " \t\n"), max + 1
}
