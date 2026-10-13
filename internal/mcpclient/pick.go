package mcpclient

import (
	"sort"
	"strings"
)

// Pick 은 창에 들어갈 만큼만 고른다.
//
// REA 한 서버만 해도 도구 170개의 스키마가 약 931,000 토큰이다 — 우리 창
// (16,384)의 **57배**다. 가장 큰 도구 하나가 이미 창 두 배다. 전부 알리는 길은
// 없으니, 할 일에 맞는 몇 개만 준다.
//
// 상용 에이전트도 같은 일을 한다 — REA 가 「Route the target first」 표를 담은
// 지침을 따로 설치하는 까닭이 이것이다.
//
// maxChars 는 글자 수다. 부르는 쪽이 토큰 예산을 글자로 바꿔서 넘긴다.
func Pick(tools []Tool, want []string, maxChars int) []Tool {
	if len(tools) == 0 || maxChars <= 0 {
		return nil
	}
	terms := terms(want)

	type scored struct {
		t Tool
		n int
		i int
	}
	var all []scored
	for i, t := range tools {
		all = append(all, scored{t: t, n: score(t, terms), i: i})
	}
	// 점수가 높은 것부터, 같으면 원래 차례대로.
	sort.SliceStable(all, func(a, b int) bool {
		if all[a].n != all[b].n {
			return all[a].n > all[b].n
		}
		return all[a].i < all[b].i
	})

	var out []Tool
	used := 0
	for _, s := range all {
		// 안 맞는 것은 넣지 않는다 — 엉뚱한 도구를 주는 것은 안 주느니만 못하다.
		// 다만 고를 말이 없으면(terms 가 비면) 차례대로 채운다.
		if len(terms) > 0 && s.n == 0 {
			break
		}
		c := cost(s.t)
		if used+c > maxChars {
			continue // 큰 것 하나 때문에 뒤의 작은 것까지 버리지 않는다
		}
		out = append(out, s.t)
		used += c
	}
	return out
}

// cost 는 그 도구를 알리는 데 드는 글자 수다.
func cost(t Tool) int {
	return len(t.Name) + len(t.Description) + len(t.Schema)
}

func terms(want []string) []string {
	var out []string
	for _, w := range want {
		for _, f := range strings.FieldsFunc(strings.ToLower(w), func(r rune) bool {
			return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
		}) {
			if len(f) > 2 {
				out = append(out, f)
			}
		}
	}
	return out
}

// score 는 이름에 더 무겁게 준다 — 설명은 길어서 우연히 걸리기 쉽다.
func score(t Tool, terms []string) int {
	name := strings.ToLower(t.Name)
	desc := strings.ToLower(t.Description)
	n := 0
	for _, term := range terms {
		if strings.Contains(name, term) {
			n += 3
		}
		if strings.Contains(desc, term) {
			n++
		}
	}
	return n
}
