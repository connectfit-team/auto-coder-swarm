package agentloop

import (
	"sort"
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
	"github.com/connectfit-team/auto-coder-swarm/internal/mcpclient"
)

// pickSections 은 지침에서 **쓸 절만** 골라 싣는다.
//
// 앞에서부터 자르면 안 된다. REA 의 SKILL.md 는 13,580바이트인데 우리 창은
// 16,384 다. 앞 1,500바이트는 「서버를 어떻게 등록하나」 — 이미 붙어 있는
// 우리에겐 쓸모가 없고, 정작 필요한 「무엇부터 열어라」 표(2211~7283바이트)는
// 한가운데서 잘렸다. 그래서 모델이 open_binary 전에 다른 것을 불렀다.
//
// 잣대는 하나다 — **그 절이 우리가 실제로 가진 도구를 몇 개나 이름으로
// 부르는가.** 하나도 안 부르는 절은 도구 고르는 데 도움이 안 된다.
func pickSections(body string, tools []mcpclient.Tool, budget int) string {
	names := make([]string, 0, len(tools))
	for _, t := range tools {
		if len(t.Name) > 3 {
			names = append(names, t.Name)
		}
	}
	if len(names) == 0 {
		out, _ := agent.ClipToTokens(body, budget)
		return out
	}

	type section struct {
		at   int
		text string
		hits int
	}
	var secs []section
	for i, raw := range strings.Split("\n"+body, "\n## ") {
		text := raw
		if i > 0 {
			text = "## " + raw
		}
		if strings.TrimSpace(text) == "" {
			continue
		}
		seen := map[string]bool{}
		for _, n := range names {
			if !seen[n] && strings.Contains(text, n) {
				seen[n] = true
			}
		}
		secs = append(secs, section{at: i, text: text, hits: len(seen)})
	}
	if len(secs) == 0 {
		out, _ := agent.ClipToTokens(body, budget)
		return out
	}

	// 머리말(첫 덩어리)은 무엇을 하는 지침인지 알려 주므로 늘 넣되 짧게 줄인다.
	head, _ := agent.ClipToTokens(secs[0].text, budget/5)
	used := agent.EstimateTokens(head)

	rest := secs[1:]
	sort.SliceStable(rest, func(a, b int) bool { return rest[a].hits > rest[b].hits })

	keep := map[int]string{0: head}
	for _, s := range rest {
		if s.hits == 0 {
			continue // 도구를 하나도 안 부르는 절은 넣지 않는다
		}
		n := agent.EstimateTokens(s.text)
		if used+n > budget {
			continue // 큰 절 하나 때문에 뒤의 작은 절을 버리지 않는다
		}
		keep[s.at] = s.text
		used += n
	}

	var order []int
	for i := range keep {
		order = append(order, i)
	}
	sort.Ints(order)

	var b strings.Builder
	for _, i := range order {
		b.WriteString(strings.TrimRight(keep[i], "\n") + "\n\n")
	}
	return strings.TrimRight(b.String(), "\n")
}
