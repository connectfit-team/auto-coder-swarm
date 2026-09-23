package orchestrator

import (
	"fmt"
	"regexp"
	"strings"
)

// 코드를 그대로 옮긴 주석을 잡는다.
//
// 팀의 주석 규약(code-comments/SKILL.md)은 프롬프트에 실린다 — 실측 W-13896
// 에서도 실렸다. 그런데도 이런 것이 나왔다.
//
//	// RequestUpdateReceivedRequest 메시지 정의
//	message RequestUpdateReceivedRequest {
//
//	// 내부 서비스에 UpdateReceivedRequest RPC를 추가한다
//	rpc UpdateReceivedRequest(...) returns (...) {}
//
// 규약을 넣어도 안 지킨다는 것은 이 저장소가 이미 적어 둔 사실이다(skills.go).
// 그래서 기계로 검사할 수 있는 것은 검사한다.
//
// **좁게 잡는다.** 선언 바로 위의 주석이, 그 선언의 이름을 뺀 나머지가
// 뼈대 낱말(정의·추가·메시지·서비스…)뿐일 때만 문다. 무엇이든 한 마디라도
// 더 적혀 있으면 두고 본다 — 멀쩡한 주석을 물면 시도만 깎는다.

var (
	reDeclLine = regexp.MustCompile(`^\s*(?:message|enum|service|rpc|func|type)\s+([A-Za-z_][A-Za-z0-9_]*)`)
	reComment  = regexp.MustCompile(`^\s*(?://+|#)\s*(.*)$`)
	reWordSep  = regexp.MustCompile(`[^\p{L}\p{N}]+`)
)

// 뼈대 낱말. 이것만 남으면 코드를 옮겨 적은 것이다.
var boilerplateWords = map[string]bool{
	"정의": true, "선언": true, "추가": true, "생성": true, "구현": true,
	"메시지": true, "필드": true, "서비스": true, "요청": true, "응답": true,
	"내부": true, "새": true, "새로": true, "함수": true, "타입": true, "구조체": true,
	"한다": true, "합니다": true, "추가한다": true, "정의한다": true, "만든다": true,
	"위한": true, "위해": true, "대한": true,
	"message": true, "enum": true, "service": true, "rpc": true, "func": true,
	"type": true, "struct": true, "interface": true, "definition": true,
	"define": true, "defines": true, "add": true, "adds": true, "new": true,
	"the": true, "a": true, "an": true, "for": true, "to": true, "of": true,
}

// 뒤에 붙는 조사. 붙은 채로는 뼈대 낱말인지 알 수 없다.
var particles = []string{"으로", "에서", "에게", "이라", "라는", "를", "을", "는", "은", "이", "가", "의", "에", "로", "와", "과", "도"}

// restatingComments 는 선언을 그대로 옮겨 적은 주석을 준다.
func restatingComments(diff string) []string {
	var out []string
	var pending []string // 바로 앞에 새로 붙인 주석들

	for _, line := range strings.Split(diff, "\n") {
		if !strings.HasPrefix(line, "+") || strings.HasPrefix(line, "+++") {
			pending = nil
			continue
		}
		body := line[1:]
		if m := reComment.FindStringSubmatch(body); m != nil {
			if t := strings.TrimSpace(m[1]); t != "" {
				pending = append(pending, t)
			}
			continue
		}
		m := reDeclLine.FindStringSubmatch(body)
		if m == nil {
			if strings.TrimSpace(body) != "" {
				pending = nil
			}
			continue
		}
		for _, c := range pending {
			if restatesDecl(c, m[1]) {
				out = append(out, fmt.Sprintf(
					"주석 %q 가 바로 아래 %s 를 그대로 옮겨 적었다. "+
						"코드를 읽어서 알 수 있는 것은 쓰지 않는다 — 왜 그렇게 했는지나 "+
						"무엇이 조용히 깨지는지를 적거나, 아니면 지워라.", c, m[1]))
			}
		}
		pending = nil
	}
	return out
}

// restatesDecl 은 그 주석이 선언 이름과 뼈대 낱말뿐인지 본다.
func restatesDecl(comment, name string) bool {
	words := reWordSep.Split(comment, -1)
	meaningful := 0
	sawName := false
	for _, w := range words {
		if w == "" {
			continue
		}
		base := trimParticle(w)
		if strings.EqualFold(base, name) || strings.EqualFold(w, name) {
			sawName = true
			continue
		}
		if boilerplateWords[strings.ToLower(base)] || boilerplateWords[strings.ToLower(w)] {
			continue
		}
		meaningful++
	}
	// 이름을 안 적은 주석은 옮겨 적은 것이 아니다.
	return sawName && meaningful == 0
}

func trimParticle(w string) string {
	for _, p := range particles {
		if len([]rune(w)) > len([]rune(p)) && strings.HasSuffix(w, p) {
			return strings.TrimSuffix(w, p)
		}
	}
	return w
}
