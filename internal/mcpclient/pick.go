package mcpclient

import (
	"encoding/json"
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
		if in := inputLine(h.t.Schema); in != "" {
			b.WriteString("  " + in + "\n")
		}
	}
	// **찾은 다음에 무엇을 할지 함께 적는다.** 실측에서 모델이 find_tools 만
	// 네 번 돌며 제자리를 돌았다 — 목록만 주면 다음 걸음을 모른다.
	b.WriteString("\n이 가운데 하나를 **바로 불러라.** find_tools 를 다시 부르지 마라.")
	return b.String()
}

// inputLine 은 입력 속성 **이름만** 적는다.
//
// codex 가 같은 일을 한다(`mcp.rs:520 build_mcp_search_text`) — 검색 결과에서
// 전체 스키마는 떼어 내고(`input_schema = None`) 속성 이름을 정렬해 싣는다.
//
// 이름만 알려 주면 모델은 인자를 몰라 못 부른다. 실측에서 찾아만 놓고 인자가
// 없는 도구를 부르거나, 목록을 그대로 답으로 냈다.
func inputLine(schema json.RawMessage) string {
	var s struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	if len(schema) == 0 || json.Unmarshal(schema, &s) != nil || len(s.Properties) == 0 {
		return ""
	}
	var names []string
	for k := range s.Properties {
		names = append(names, k)
	}
	sort.Strings(names)
	out := "입력: " + strings.Join(names, ", ")
	if len(s.Required) > 0 {
		sort.Strings(s.Required)
		out += " (필수: " + strings.Join(s.Required, ", ") + ")"
	}
	return out
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

// Overview 는 도구 이름의 갈래만 짧게 보여 준다.
//
// 미루기(Deferred)로 돌리면 모델은 카탈로그의 **낱말을 모른다.** 실측에서
// 6턴 가운데 3턴을 find_tools 로 더듬는 데 썼고, 첫 질의는 96자만 건졌다.
//
// 스키마는 931,000 토큰이라 못 보여 주지만, **앞머리만 세어 보여 주는 것은
// 몇백 자면 된다.** REA 자신도 「Route the target first」 표를 지침에 담아
// 같은 일을 한다.
func Overview(tools []Tool, maxGroups int) string {
	if len(tools) == 0 {
		return ""
	}
	if maxGroups <= 0 {
		maxGroups = 14
	}
	n := map[string]int{}
	for _, t := range tools {
		head := t.Name
		if i := strings.IndexByte(head, '_'); i > 0 {
			head = head[:i]
		}
		n[head]++
	}
	var heads []string
	for h := range n {
		heads = append(heads, h)
	}
	sort.Slice(heads, func(a, b int) bool {
		if n[heads[a]] != n[heads[b]] {
			return n[heads[a]] > n[heads[b]]
		}
		return heads[a] < heads[b]
	})

	var parts []string
	for i, h := range heads {
		if i >= maxGroups {
			break
		}
		parts = append(parts, fmt.Sprintf("%s(%d)", h, n[h]))
	}
	// **도구 이름처럼 보이면 안 된다.** 실측에서 모델이 갈래 표기 `binary_*` 를
	// 도구 이름으로 알고 `binary_` 를 불렀다.
	return "[자주 나오는 낱말 — find_tools 에 물을 때 쓴다. **도구 이름이 아니다**]\n" +
		strings.Join(parts, " · ") +
		"\n이 낱말을 그대로 부르지 마라. find_tools 로 찾아 나온 **정확한 이름**만 부른다."
}

// InputHint 는 그 도구가 받는 입력 이름을 한 줄로 준다.
//
// 모델은 **인자 이름을 지어낸다.** 실측에서 open_binary 를 target_path 로,
// search_procedures 를 procedure_name 으로 불렀다(둘 다 틀렸다). 서버가
// 「Unrecognized key」 라고만 하면 모델은 또 지어낸다 — 맞는 이름을 함께 준다.
func InputHint(tools []Tool, name string) string {
	for _, t := range tools {
		if t.Name != name {
			continue
		}
		if in := inputLine(t.Schema); in != "" {
			return name + " 의 " + in
		}
		return ""
	}
	return ""
}

// InputsMentionedIn 은 글 안에 이름이 나온 도구들의 입력을 모아 준다.
//
// 절차(prompt)는 「open_binary 로 열고 search_procedures 로 찾아라」 처럼
// **이름만** 일러 준다. 스키마를 못 본 모델은 인자를 지어낸다.
func InputsMentionedIn(tools []Tool, text string) string {
	if text == "" {
		return ""
	}
	var lines []string
	seen := map[string]bool{}
	for _, t := range tools {
		if seen[t.Name] || !strings.Contains(text, t.Name) {
			continue
		}
		seen[t.Name] = true
		if in := inputLine(t.Schema); in != "" {
			lines = append(lines, "- "+t.Name+" 의 "+in)
		}
		if len(lines) >= 12 {
			break
		}
	}
	if len(lines) == 0 {
		return ""
	}
	sort.Strings(lines)
	return "\n\n[위에 나온 도구가 받는 입력 — 지어내지 마라]\n" + strings.Join(lines, "\n")
}
