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

// **밖에서 부를 수 있는 이름**만 센다.
//
// 함수 안의 지역 변수 하나로는 기능이 생기지 않는다. 한 요청이 이 한 줄로
// 모든 관문을 통과했다(W-80975).
//
//	const known = new Set((connectable?.staffs ?? []).map((s) => s.workplaceId ?? ''));
//
// 새 선언이긴 하다. 그런데 연결 보류와 아무 상관이 없다. 없는 기능을
// 만들라고 했으면 **함수·타입·열거 값처럼 이름이 붙은 것**이 생겨야 한다.
var addedDeclRe = []*regexp.Regexp{
	// 함수·타입·클래스 선언은 들여쓰기와 무관하게 센다.
	regexp.MustCompile(`^\s*(?:export\s+)?(?:declare\s+)?(?:async\s+)?(?:function|class|interface|type|enum)\s+\w+`),
	// 값 선언은 **내보낸 것만** 센다 — 지역 변수는 기능이 아니다.
	regexp.MustCompile(`^\s*export\s+(?:const|let|var)\s+\w+`),
	// Go 는 맨 왼쪽 선언만 센다.
	regexp.MustCompile(`^(?:func|type)\s+\(?[^)]*\)?\s*\w+`),
	regexp.MustCompile(`^(?:const|var)\s+\w+`),
	// 열거·객체에 새 값 (HOLD = 'hold', · hold: true, · case 'hold':)
	regexp.MustCompile(`^\s*\w+\s*[:=]\s*['"\x60][\w.-]+['"\x60]`),
	regexp.MustCompile(`^\s*case\s+['"\x60][\w.-]+['"\x60]`),
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

	// 더한 줄만 모아 둔다 — 껍데기인지 보려면 뒤따르는 줄이 필요하다.
	var addedBody []string
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			addedBody = append(addedBody, line[1:])
		}
	}

	var out []string
	var kept []int
	var bodies []string
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
				kept = append(kept, len(out))
				out = append(out, trimmed)
				bodies = append(bodies, body)
				break
			}
		}
	}
	// 빈 껍데기만 남았으면 만든 것이 아니다.
	var real []string
	for i, name := range out {
		_ = i
		idx := indexOfLine(addedBody, bodies[i])
		if idx >= 0 && stubAt(addedBody, idx) {
			continue
		}
		real = append(real, name)
	}
	_ = kept
	return real
}

func indexOfLine(lines []string, want string) int {
	for i, l := range lines {
		if l == want {
			return i
		}
	}
	return -1
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

// 빈 껍데기는 만든 것이 아니다.
//
// 「기능을 추가할거야」 에 이것이 넘어왔다(W-58244).
//
//	export function useI18n() {
//	    // Implementation of useI18n function
//	}
//
// 이름은 생겼다. 그래서 「새 이름이 0개」 관문을 지나갔다. 그런데 속이 비어
// 있으니 아무 일도 하지 않는다. 주석으로 「여기에 구현」 이라고 적은 것은
// 구현이 아니다.
//
// 선언을 셀 때 **속이 있는지**까지 본다. 중괄호를 열었으면, 닫히기 전에
// 주석·빈 줄이 아닌 줄이 하나는 있어야 한다.

var reDeclOpensBody = regexp.MustCompile(`^\s*(?:export\s+)?(?:declare\s+)?(?:async\s+)?(?:function|class)\s+[\p{L}\p{N}_]+[^{]*\{\s*$`)

// stubAt 은 그 줄에서 시작한 선언이 빈 껍데기인지 본다.
// 껍데기가 아니거나 판단할 수 없으면 false.
func stubAt(added []string, i int) bool {
	if !reDeclOpensBody.MatchString(added[i]) {
		return false
	}
	depth := 1
	for j := i + 1; j < len(added) && depth > 0; j++ {
		t := strings.TrimSpace(added[j])
		depth += strings.Count(t, "{") - strings.Count(t, "}")
		if depth <= 0 {
			break
		}
		if t == "" || strings.HasPrefix(t, "//") || strings.HasPrefix(t, "*") || strings.HasPrefix(t, "/*") {
			continue
		}
		return false // 속이 있다
	}
	return true
}
