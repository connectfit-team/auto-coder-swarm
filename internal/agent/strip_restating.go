package agent

import (
	"regexp"
	"strings"
)

// 코드를 그대로 옮겨 적은 주석은 **기계가 지운다.**
//
// 이것을 관문으로 막았더니(ACS#162) 계약 수정이 다 맞았는데 주석 때문에 세
// 시도를 다 쓰고 죽었다(실측 W-71222 — 두 파일 다 고치고 있던 메시지에
// 필드까지 바르게 넣은 수정이었다). 값이 안 맞는다.
//
// 지워도 잃는 것이 없다. 이 검사는 **선언 이름과 뼈대 낱말뿐인 주석**만
// 고르기 때문이다 — 한 마디라도 더 적혀 있으면 두고 본다. 팀 규약도 둘 중
// 하나를 고르라고 한다: 「왜 그렇게 했는지를 적거나, **아니면 지워라**」.
//
// 고를 것이 없으면 기계가 한다(llm-decides-machine-executes).

var (
	reDeclHere    = regexp.MustCompile(`^\s*(?:message|enum|service|rpc|func|type|const|var|class|interface)\s+([A-Za-z_][\w$]*)`)
	reCommentHere = regexp.MustCompile(`^\s*(?://+|#)\s*(.*)$`)
)

// StripRestatingComments 는 바로 아래 선언을 옮겨 적은 주석 줄을 지운다.
// 지운 줄 수를 함께 준다 — 조용히 고치지 않으려는 것이다.
func StripRestatingComments(content string) (string, int) {
	lines := strings.Split(content, "\n")
	drop := make([]bool, len(lines))
	removed := 0

	for i, line := range lines {
		m := reDeclHere.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		// 바로 위에 붙은 주석 덩어리를 거슬러 올라간다.
		for j := i - 1; j >= 0; j-- {
			c := reCommentHere.FindStringSubmatch(lines[j])
			if c == nil {
				break
			}
			if t := strings.TrimSpace(c[1]); t != "" && restatesDeclName(t, m[1]) {
				if !drop[j] {
					drop[j] = true
					removed++
				}
			}
		}
	}
	if removed == 0 {
		return content, 0
	}
	out := make([]string, 0, len(lines))
	for i, l := range lines {
		if !drop[i] {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n"), removed
}

// restatesDeclName 은 그 주석이 선언 이름과 뼈대 낱말뿐인지 본다.
// orchestrator 쪽 검사와 같은 잣대다 — 둘이 어긋나면 관문이 지운 뒤에 또 문다.
func restatesDeclName(comment, name string) bool {
	words := reNonWord.Split(comment, -1)
	meaningful, sawName := 0, false
	for _, w := range words {
		if w == "" {
			continue
		}
		base := trimKoreanParticle(w)
		if strings.EqualFold(base, name) || strings.EqualFold(w, name) {
			sawName = true
			continue
		}
		stem := trimKoreanVerbEnding(base)
		if boilerplate[strings.ToLower(base)] ||
			boilerplate[strings.ToLower(w)] ||
			boilerplate[strings.ToLower(stem)] {
			continue
		}
		meaningful++
	}
	return sawName && meaningful == 0
}

var reNonWord = regexp.MustCompile(`[^\p{L}\p{N}]+`)

var boilerplate = map[string]bool{
	"정의": true, "선언": true, "추가": true, "생성": true, "구현": true,
	"메시지": true, "필드": true, "서비스": true, "요청": true, "응답": true,
	"내부": true, "새": true, "새로": true, "함수": true, "타입": true, "구조체": true,
	"한다": true, "합니다": true, "만든다": true, "위한": true, "위해": true, "대한": true,
	"message": true, "enum": true, "service": true, "rpc": true, "func": true,
	"type": true, "struct": true, "interface": true, "definition": true,
	"define": true, "defines": true, "add": true, "adds": true, "new": true,
	"the": true, "a": true, "an": true, "for": true, "to": true, "of": true, "is": true,
}

var koreanParticles = []string{"으로", "에서", "에게", "이라", "라는", "를", "을", "는", "은", "이", "가", "의", "에", "로", "와", "과", "도"}

// 긴 것부터 뗀다. `합니다` 를 먼저 떼야 `다` 로 잘못 안 뗀다.
var koreanVerbEndings = []string{
	"하였습니다", "되었습니다", "합니다", "입니다", "됩니다", "하였다", "되었다",
	"하여야", "해야", "하여", "하고", "한다", "했다", "된다", "됐다", "하기", "되기",
}

func trimKoreanParticle(w string) string {
	for _, p := range koreanParticles {
		if len([]rune(w)) > len([]rune(p)) && strings.HasSuffix(w, p) {
			return strings.TrimSuffix(w, p)
		}
	}
	return w
}

func trimKoreanVerbEnding(w string) string {
	for _, e := range koreanVerbEndings {
		if len([]rune(w)) > len([]rune(e)) && strings.HasSuffix(w, e) {
			return strings.TrimSuffix(w, e)
		}
	}
	return w
}
