package agent

import (
	"fmt"
	"sort"
	"strings"
)

// 못 찾았다고만 말하면 다음 시도도 똑같이 틀린다.
//
// 코더는 "이 내용을 찾아 바꿔라" 로 일한다. 모델이 없는 것을 지어내면
// "원문에 없는 내용을 찾으라고 했다" 로 끝나는데, 그 말에는 **원문에 무엇이
// 있는지가 없다.** 그래서 다시 계획해도 같은 것을 지어낸다 — 실측으로
// 세 시도가 같은 자리에서 죽었다(W-70980: 있지도 않은
// DeleteAndDisconnectOnAccountDeleteWithTransaction 을 찾으라고 했다).
//
// 가장 많이 겹치는 원문 조각을 함께 준다. 이름이 비슷한 함수가 실제로 있으면
// 그것을 보고 고칠 수 있고, 아무것도 안 겹치면 그 파일이 아니라는 뜻이다.

const (
	anchorWindow = 6 // 겹침을 셀 때 볼 줄 수
	anchorShow   = 4 // 사람·모델에게 보여 줄 줄 수
)

// nearestAnchor 는 찾는 내용과 가장 많이 겹치는 원문 조각을 준다.
// 아무것도 안 겹치면 빈 문자열이다.
func nearestAnchor(srcLines []string, search string) string {
	want := identSet(search)
	if len(want) == 0 {
		return ""
	}

	best, bestAt, bestN := 0, -1, 0
	for i := 0; i < len(srcLines); i++ {
		for n := 1; n <= anchorWindow && i+n <= len(srcLines); n++ {
			got := identSet(strings.Join(srcLines[i:i+n], "\n"))
			hit := overlap(want, got)
			// 같은 만큼 겹치면 **좁은 창**을 고른다. 넓은 창은 앞줄에서
			// 시작해도 같은 점수가 나오므로, 그대로 두면 엉뚱한 줄을 짚는다.
			if hit > best || (hit == best && bestAt >= 0 && n < bestN) {
				best, bestAt, bestN = hit, i, n
			}
		}
	}
	// 절반도 안 겹치면 짚어 줄 것이 없다. 엉뚱한 곳을 가리키면 더 나쁘다.
	if bestAt < 0 || best*2 < len(want) {
		return ""
	}

	from := bestAt
	to := bestAt + bestN
	if to-from < anchorShow && to < len(srcLines) {
		to = min(len(srcLines), from+anchorShow)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "원문에서 가장 비슷한 곳은 %d줄이다 (낱말 %d/%d 겹침):\n",
		from+1, best, len(want))
	for i := from; i < to; i++ {
		fmt.Fprintf(&b, "%d: %s\n", i+1, srcLines[i])
	}
	return strings.TrimRight(b.String(), "\n")
}

// overlap 은 이름이 몇 개 겹치는지 센다.
//
// **똑같은 이름만 세면 아무것도 안 걸린다.** 모델이 지어내는 이름은 진짜
// 이름에 조각을 덧붙인 것이 많다 —
// DeleteAndDisconnectOnAccountDelete**WithTransaction** 처럼. 한쪽이 다른
// 쪽을 품고 있으면 겹친 것으로 본다. 짧은 조각으로 우연히 걸리지 않게
// 여섯 글자 이상만 그렇게 센다.
func overlap(want, got map[string]bool) int {
	hit := 0
	for w := range want {
		if got[w] {
			hit++
			continue
		}
		if len(w) < 6 {
			continue
		}
		for g := range got {
			if len(g) < 6 {
				continue
			}
			if strings.Contains(g, w) || strings.Contains(w, g) {
				hit++
				break
			}
		}
	}
	return hit
}

// identSet 은 그 글에 나오는 이름들이다. 짧은 것과 흔한 것은 뺀다 —
// func·ctx·err 로 겹침을 세면 아무 줄이나 이긴다.
func identSet(s string) map[string]bool {
	out := map[string]bool{}
	for _, f := range strings.FieldsFunc(s, func(r rune) bool {
		return !(r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9')
	}) {
		if len(f) < 4 || anchorStopWords[strings.ToLower(f)] {
			continue
		}
		out[f] = true
	}
	return out
}

var anchorStopWords = map[string]bool{
	"func": true, "return": true, "string": true, "error": true, "context": true,
	"ctx": true, "err": true, "nil": true, "bool": true, "int64": true,
	"struct": true, "interface": true, "range": true, "case": true, "switch": true,
	"import": true, "package": true, "const": true, "true": true, "false": true,
}

// sortedKeys 는 시험이 순서에 흔들리지 않게 한다.
func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
