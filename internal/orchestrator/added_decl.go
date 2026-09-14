package orchestrator

import (
	"regexp"
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/guard"
)

// 새로 만들라고 했는데 **새로 생긴 이름이 하나도 없으면** 만든 것이 아니다.
//
// 한 요청이 승인 대기까지 갔는데 넘어온 것이 이랬다(W-16333).
//
//	· 이미 있던 함수를 try/catch 로 감쌌다
//	· 파일 끝 줄바꿈을 지웠다
//	· 상관없는 파일의 주석에서 한 글자를 빼먹었다 ("이으면" → "이면")
//
// 타입 검사도 통과하고, 계획이 짚은 파일도 건드렸고, 검토자도 넘겼다.
// 셋 다 **맞게** 동작했다 — 문법도 맞고 그 파일이 맞으니까. 그런데 기능은
// 없다. 없는 것을 만들라고 했으면 **이름이 하나는 생겨야 한다.**
//
// 판단하지 않고 센다: 더한 줄에 새 선언이나 새 값이 하나라도 있는가.

var addedDeclRe = []*regexp.Regexp{
	// TS·JS — 선언
	regexp.MustCompile(`^\s*(?:export\s+)?(?:declare\s+)?(?:async\s+)?(?:function|const|let|var|class|interface|type|enum)\s+\w+`),
	// Go — 선언
	regexp.MustCompile(`^\s*(?:func|type|const|var)\s+\(?[^)]*\)?\s*\w+`),
	// 열거·객체에 새 값 (HOLD = 'hold', · hold: true, · case 'hold':)
	regexp.MustCompile(`^\s*\w+\s*[:=]\s*['"\x60][\w.-]+['"\x60]`),
	regexp.MustCompile(`^\s*case\s+['"\x60][\w.-]+['"\x60]`),
	// Dart·Svelte 속성 선언
	regexp.MustCompile(`^\s*(?:final|late|var)\s+\w+\s+\w+`),
}

// addedDeclarations 는 diff 가 **새로 들인 이름**을 준다.
//
// 주석·빈 줄은 안 센다. **옮기거나 들여쓰기만 바뀐 줄도 안 센다** — 지운 줄에
// 같은 내용이 있으면 새것이 아니다. try/catch 로 감싸면서 안쪽 줄이 통째로
// 밀리는데, 그것을 새 선언으로 세면 아무것도 안 만들고도 통과한다(W-16333).
func addedDeclarations(diff string) []string {
	removed := map[string]bool{}
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			removed[strings.TrimSpace(line[1:])] = true
		}
	}

	var out []string
	for _, line := range strings.Split(diff, "\n") {
		if !strings.HasPrefix(line, "+") || strings.HasPrefix(line, "+++") {
			continue
		}
		body := strings.TrimRight(line[1:], " \t\r")
		trimmed := strings.TrimSpace(body)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "*") ||
			strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if removed[trimmed] {
			continue // 옮겼거나 들여쓰기만 바뀌었다
		}
		for _, re := range addedDeclRe {
			if re.MatchString(body) {
				out = append(out, trimmed)
				break
			}
		}
	}
	return out
}

// CheckNewFeatureAddedSomething 은 새로 만드는 일에서만 본다.
func CheckNewFeatureAddedSomething(diff string) []guard.Violation {
	if strings.TrimSpace(diff) == "" {
		return nil
	}
	if len(addedDeclarations(diff)) > 0 {
		return nil
	}
	return []guard.Violation{{
		Why: "새로 만들라고 했는데 새로 생긴 이름이 하나도 없다 — 있던 코드를 감싸거나 손질했을 뿐이다",
		Evidence: clipList(append([]string{"고친 파일: " + strings.Join(changedFiles(diff), " · ")},
			"새 선언·새 값: 0개")),
	}}
}
