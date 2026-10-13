package main

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/mcpclient"
)

// verify 는 붙은 서버가 내놓는 것을 **우리가 다 받고 있는지** 전수로 본다.
//
// LLM 을 쓰지 않는다 — 측정이 도는 중에도 돌릴 수 있고, 답이 흔들리지 않는다.
// 「완벽히 이식됐다」 는 말은 이걸 돌려 보고 하는 것이다.
func verify(ctx context.Context, clients []*mcpclient.Client) int {
	tools, owner := mcpclient.AllTools(clients)
	prompts, promptOwner := mcpclient.AllPrompts(clients)

	bad := 0
	say := func(ok bool, format string, a ...any) {
		mark := "✓"
		if !ok {
			mark = "✗"
			bad++
		}
		fmt.Printf("%s %s\n", mark, fmt.Sprintf(format, a...))
	}

	for _, c := range clients {
		fmt.Printf("\n[%s]\n", c.Name())
		say(len(c.Tools()) > 0, "도구 %d개", len(c.Tools()))
		say(true, "작업 절차 %d개", len(c.Prompts()))
		say(true, "쓰는 법 %d자", len(c.Instructions()))
	}

	fmt.Printf("\n[전수 검사]\n")

	// 1. 이름으로 다 부를 수 있나.
	var unreachable []string
	for _, t := range tools {
		if _, ok := owner[t.Name]; !ok {
			unreachable = append(unreachable, t.Name)
		}
	}
	say(len(unreachable) == 0, "이름으로 부를 수 있다: %d/%d%s",
		len(tools)-len(unreachable), len(tools), listIf(unreachable))

	// 2. 안 알리는 것도 find_tools 로 찾을 수 있나.
	//    Visible 에서 빠진 것은 **숨긴 것이 아니다.** 찾을 수 없으면 숨긴 것이다.
	visible, oversized := mcpclient.Visible(tools)
	var lost []string
	for _, name := range oversized {
		if !strings.Contains(mcpclient.FindTools(tools, name, 50), name) {
			lost = append(lost, name)
		}
	}
	say(len(lost) == 0, "안 알리는 %d개도 find_tools 로 찾힌다 (알리는 것 %d개)%s",
		len(oversized), len(visible), listIf(lost))

	// 3. 스키마가 있나 — 없으면 인자를 몰라 못 부른다.
	var noSchema []string
	for _, t := range tools {
		if len(t.Schema) == 0 {
			noSchema = append(noSchema, t.Name)
		}
	}
	say(len(noSchema) == 0, "스키마가 있다: %d/%d%s",
		len(tools)-len(noSchema), len(tools), listIf(noSchema))

	// 4. 작업 절차를 다 받아 올 수 있나.
	var failed []string
	for _, p := range prompts {
		c := promptOwner[p.Name]
		if c == nil {
			failed = append(failed, p.Name+"(임자 없음)")
			continue
		}
		// 절차에는 필수 인자가 있다 — 비우면 받아 올 수 있는지 못 본다.
		args := map[string]any{}
		for _, a := range p.Arguments {
			if a.Required {
				args[a.Name] = "verify"
			}
		}
		body, err := c.GetPrompt(ctx, p.Name, args)
		if err != nil || strings.TrimSpace(body) == "" {
			failed = append(failed, p.Name)
		}
	}
	say(len(failed) == 0, "작업 절차를 받아 온다: %d/%d%s",
		len(prompts)-len(failed), len(prompts), listIf(failed))

	// 5. 쓰는 법을 하나라도 받았나.
	say(mcpclient.AllInstructions(clients) != "", "쓰는 법을 받았다")

	fmt.Println()
	if bad == 0 {
		fmt.Println("다 받고 있다.")
	} else {
		fmt.Printf("못 받는 것이 %d가지 있다.\n", bad)
	}
	return bad
}

func listIf(xs []string) string {
	if len(xs) == 0 {
		return ""
	}
	sort.Strings(xs)
	if len(xs) > 6 {
		xs = append(xs[:6], "…")
	}
	return " — 빠진 것: " + strings.Join(xs, ", ")
}
