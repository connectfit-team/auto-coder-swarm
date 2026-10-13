package mcpclient

import (
	"fmt"
	"sort"
	"strings"
)

// 도구 하나가 쓸 수 있는 글자. 넘으면 숨긴다.
//
// codex 가 같은 자리에 쓰는 값은 도구 하나 8,000바이트·전부 64,000바이트인데,
// 그건 200k 창을 전제한 것이다. 우리 창은 16,384 라 그 비례로 줄인다.
// REA 의 compare_web_captures 는 혼자 75,501자 — 창의 두 배다. 이런 것은
// 고르기 전에 **숨긴다**(codex 의 ToolExposure::Hidden).
const (
	MaxToolChars  = 2500
	MaxTotalChars = 5000
)

// Visible 은 혼자서 창을 먹는 도구를 걸러 낸다.
func Visible(tools []Tool) (ok []Tool, hidden []string) {
	for _, t := range tools {
		if cost(t) > MaxToolChars {
			hidden = append(hidden, t.Name)
			continue
		}
		ok = append(ok, t)
	}
	return ok, hidden
}

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
	// **고를 말이 없으면 아무것도 주지 않는다.**
	//
	// 앞에서부터 채우면 「골랐다」 는 얼굴로 엉뚱한 것을 준다. 실측에서
	// 한국어 목표로 돌렸더니 영어 카탈로그와 맞는 낱말이 하나도 없어, 170개
	// 가운데 그냥 앞의 6개가 나갔다. 못 고른 것은 **못 고른 것**이다 —
	// 그때는 FindTools 로 모델이 직접 찾게 한다.
	if len(terms) == 0 {
		return nil
	}

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
		if s.n == 0 {
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

// FindTools 는 이름·설명만 짧게 돌려준다 — 스키마는 뺀다.
//
// 카탈로그가 170개(스키마로 치면 창의 57배)라 다 알릴 수 없다. 모델이 먼저
// 찾아보고 쓸 것을 고르게 한다 — 상용 에이전트가 큰 카탈로그를 다루는 방식이고,
// REA 자신도 「Route the target first」 표로 같은 일을 한다.
func FindTools(tools []Tool, query string, max int) string {
	if max <= 0 {
		max = 20
	}
	t := terms([]string{query})
	if len(t) == 0 {
		return "찾을 말이 없다. 영어 낱말로 다시 물어라 (예: decompile, capabilities, android)."
	}
	type hit struct {
		t Tool
		n int
	}
	var hits []hit
	for _, x := range tools {
		if n := score(x, t); n > 0 {
			hits = append(hits, hit{x, n})
		}
	}
	sort.SliceStable(hits, func(a, b int) bool { return hits[a].n > hits[b].n })
	if len(hits) == 0 {
		return "맞는 도구가 없다."
	}
	var b strings.Builder
	for i, h := range hits {
		if i >= max {
			fmt.Fprintf(&b, "…그 밖에 %d개 더 있다. 더 좁혀서 물어라.\n", len(hits)-max)
			break
		}
		fmt.Fprintf(&b, "- %s: %s\n", h.t.Name, firstLine(h.t.Description))
	}
	// **찾은 다음에 무엇을 할지 함께 적는다.** 실측에서 모델이 find_tools 만
	// 네 번 돌며 제자리를 돌았다 — 목록만 주면 다음 걸음을 모른다.
	b.WriteString("\n이 가운데 하나를 **바로 불러라.** find_tools 를 다시 부르지 마라.")
	return b.String()
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 110 {
		s = s[:110] + "…"
	}
	return s
}
