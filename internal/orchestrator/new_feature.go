package orchestrator

import (
	"fmt"
	"regexp"
	"strings"
)

// 「고쳐라」 와 「만들어라」 는 다른 일이다.
//
// 분석이 "그 기능을 못 찾았다" 고 하면 고칠 것이 없다는 뜻이다. 고장 요청에
// 그것은 멈출 신호다 — 없는 것을 지어내면 있지도 않은 함수를 찾으라는 계획이
// 나온다(W-70980).
//
// 그런데 **새 기능을 만들라는 요청에는 "못 찾았다" 가 당연하다.** 없으니까
// 만드는 것이다. 거기서 멈추면 사람이 시킨 일을 아예 못 한다.
//
// 그래서 요청의 성격을 먼저 가른다. 새로 만드는 일이면 멈추지 않고,
//   - 사내지식을 반드시 프롬프트에 싣고(없으면 정책을 모른 채 설계한다)
//   - **가장 비슷한 기존 코드**를 본떠 만들라고 이른다
// 는 조건을 붙여 이어 간다. 무에서 지어내는 것과 본떠 만드는 것은 다르다.

var newFeatureRe = regexp.MustCompile(
	`추가(해|하|할|한다|해줘|하자)|만들(어|자|어줘)|신규|새로\s*(만|추가)|구현(해|하|할)|넣어\s*줘|도입`)

var fixRe = regexp.MustCompile(
	`고쳐|고치|버그|안\s*되|안\s*돼|빠진|누락|틀리|잘못|실패|오류|이상해|원인`)

// IsNewFeatureRequest 는 요청이 「없는 것을 만들라」 인지 본다.
//
// 고치라는 말이 함께 있으면 고장으로 본다 — "버그 고치고 로그도 추가해줘" 는
// 고장 요청이다. 있는 코드를 다루는 일이라 없는 것을 지어낼 위험이 다르다.
func IsNewFeatureRequest(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" {
		return false
	}
	if fixRe.MatchString(t) {
		return false
	}
	return newFeatureRe.MatchString(t)
}

// CoderNewFeatureHint 는 **코더에게** 붙일 조건이다.
//
// 계획 쪽에는 조건을 붙였지만 코더는 그것을 못 봤다. 코더는 "찾아 바꿔라"
// 형식으로 일하는데, 새 기능에는 바꿀 원문이 없다. 그래서 **있지도 않은
// "고치기 전" 코드를 지어내 SEARCH 에 적는다.** 실측(W-34685)으로 네 파일이
// 모두 그렇게 죽었다.
//
//	원문에 없는 내용을 찾으라고 했다:
//	    updates := map[string]interface{}{
//	        "status": "pending",
//	    }
//	겹치는 이름이 거의 없다 — 이 파일이 아닐 수 있다.
//
// 길은 하나뿐이다. **있는 것을 닻으로 삼고, 그 옆에 새것을 함께 적는다.**
// 그러면 찾아 바꾸기 기계가 그대로 받아 준다.
func CoderNewFeatureHint() string {
	return "[새로 만드는 일이다 — 이 파일에 그 기능은 아직 없다]\n" +
		"그러니 **고치기 전 코드를 지어내지 마라.** SEARCH 에는 원문에 실제로 있는 줄만 적는다.\n" +
		"새것을 넣는 방법은 이것뿐이다:\n" +
		"  1. 가장 비슷한 **기존 선언 하나**를 통째로 SEARCH 에 적는다(원문 그대로).\n" +
		"  2. REPLACE 에 **그 선언을 그대로 다시 적고, 그 뒤에 새 선언을 붙인다.**\n" +
		"즉 REPLACE 는 SEARCH 를 품는다. 이렇게 하면 지울 것 없이 더해진다.\n" +
		"새것은 온전한 선언이어야 한다 — func 하나, type 하나, const( … ) 묶음 하나.\n" +
		"조각(if 문 하나, 대입 한 줄)만 내면 그 자리는 버려진다.\n\n"
}

// NewFeatureBrief 는 새로 만드는 일에 붙일 조건이다.
//
// 프롬프트에 그대로 실린다. 「본떠 만들라」 가 핵심이다 — 무에서 지어내면
// 없는 이름을 부르는 코드가 나온다.
func NewFeatureBrief(request, knowledge string, candidates []string) string {
	var b strings.Builder
	b.WriteString("[새로 만드는 일이다]\n")
	b.WriteString("이 저장소에 그 기능은 아직 없다. 분석이 못 찾은 것이 맞다.\n")
	b.WriteString("그러니 **고칠 자리를 찾지 말고, 가장 비슷한 기존 코드를 본떠 새로 만든다.**\n\n")
	b.WriteString("지켜야 할 것:\n")
	b.WriteString("1. 원문에 있는 코드만 닻으로 쓴다. 없는 함수·타입을 찾으라고 하면 그 자리는 버려진다.\n")
	b.WriteString("2. 새 파일을 만들어도 된다. 다만 그 폴더는 실제로 있어야 한다.\n")
	b.WriteString("3. 이름·구조는 같은 저장소의 이웃 코드와 같은 꼴로 맞춘다.\n")
	b.WriteString("4. proto 메시지·필드가 필요하면 코드에 지어내지 말고 사람에게 넘긴다고 적는다.\n")
	if knowledge != "" {
		b.WriteString("\n[사내지식 — 이 말이 요청의 뜻을 정한다]\n")
		b.WriteString(clip(knowledge, 2500))
		b.WriteString("\n")
	} else {
		b.WriteString("\n[사내지식을 못 받았다] 정책·과거 결정을 모른 채 설계하는 것이다.\n")
		b.WriteString("요청의 뜻이 갈리면 지어내지 말고 사람에게 넘긴다고 적는다.\n")
	}
	if len(candidates) > 0 {
		b.WriteString("\n[본뜰 만한 이웃 코드]\n")
		for i, c := range candidates {
			if i >= 12 {
				break
			}
			fmt.Fprintf(&b, "- %s\n", c)
		}
	}
	return b.String()
}
