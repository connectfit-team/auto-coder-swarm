// Package agentloop 은 도구를 쥔 에이전트 한 판을 돈다.
//
// 상용 에이전트(codex·Claude Code·aider)와 같은 모양이다 — 도구를 알리고,
// 궤적을 쌓고, 결과를 **같은 대화로** 되돌리고, 예산으로 멈춘다.
//
// 다른 점은 창이 16,384 토큰이라는 것뿐이다. 그래서 도구를 추리고(mcpclient.Pick)
// 궤적을 눌러야 한다(trim). 상용 에이전트는 200k 를 전제하므로 그대로는 못 쓴다.
package agentloop

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/adk/model"
	"google.golang.org/genai"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
	"github.com/connectfit-team/auto-coder-swarm/internal/mcpclient"
)

// Dispatch 는 도구 하나를 실제로 부른다.
type Dispatch func(ctx context.Context, name string, args map[string]any) (string, error)

type Loop struct {
	LLM   model.LLM
	Tools []mcpclient.Tool
	Call  Dispatch

	// Instructions 는 도구 서버가 스스로 알려 준 쓰는 법이다. 버리면 모델이
	// 서버가 아는 것을 모른 채 더듬는다.
	Instructions string

	// Prompts 는 서버가 알린 **작업 절차**다. 도구 하나가 아니라 일 전체의
	// 차례라 값이 크다 — REA 는 investigate_feature·trace_crash 같은 것을 6개 준다.
	// Skills 는 서버가 파일로 깔아 둔 **쓰는 차례**다. MCP 로는 안 온다.
	Skills []Skill

	Prompts   []mcpclient.Prompt
	GetPrompt func(ctx context.Context, name string, args map[string]any) (string, error)

	MaxTurns     int // 0 이면 8
	ToolChars    int // 도구를 알리는 데 쓸 글자. 0 이면 창의 1/4
	ResultTokens int // 도구 결과 하나가 쓸 수 있는 토큰. 0 이면 1200

	// DeferAll 은 도구 스키마를 미리 알리지 않고 find_tools 로만 찾게 한다.
	// codex 가 검색 도구를 켰을 때 하는 것과 같다.
	DeferAll bool

	// Summarize 는 접을 차례를 줄인다. 비우면 같은 LLM 에게 맡긴다.
	Summarize func(ctx context.Context, text string) (string, error)

	Log func(kind, msg string)
}

// Run 은 목표 하나를 좇는다. 모델이 도구를 그만 부르면 그때의 글이 답이다.
func (l *Loop) Run(ctx context.Context, system, goal string) (string, error) {
	maxTurns := l.MaxTurns
	if maxTurns <= 0 {
		maxTurns = 8
	}
	toolChars := l.ToolChars
	if toolChars <= 0 {
		toolChars = agent.InputTokenBudget() // 토큰 -> 글자는 넉넉히 2.5배
		toolChars = toolChars * 2 / 4
	}
	resultTokens := l.ResultTokens
	if resultTokens <= 0 {
		resultTokens = 1200
	}

	// **찾는 도구를 늘 쥐여 준다.**
	//
	// 카탈로그가 170개면 스키마만으로 창의 57배다. 골라 준 것이 빗나가도
	// 모델이 스스로 찾을 길이 있어야 한다 — 상용 에이전트가 큰 카탈로그를
	// 다루는 방식이다. 고를 말이 없을 때(한국어 목표에 영어 카탈로그) Pick 은
	// 아무것도 안 주므로, 그때는 이것이 유일한 길이다.
	// **추측하지 않는다 — 찾게 한다.**
	//
	// codex 는 검색 도구가 켜져 있으면 MCP 도구를 **전부** Deferred 로 돌린다
	// (`mcp_tool_exposure.rs`: `if search_tool_enabled { ToolExposure::Deferred }`).
	// 스키마를 프롬프트에 넣지 않고 모델이 찾아 쓰게 한다.
	//
	// 우리도 그래야 한다. 실측에서 Pick 이 한국어 목표로 엉뚱한 6개를 골라
	// 턴을 버렸다. 골라 주는 것은 **맞을 때만** 이득이고, 틀리면 손해다.
	visible, hidden := mcpclient.Visible(l.Tools)
	if len(hidden) > 0 {
		// 「숨긴다」 가 아니다 — 알리지 않을 뿐 find_tools 로 찾고 부를 수 있다.
		l.log("TOOLS", fmt.Sprintf("혼자 창을 먹는 도구 %d개는 안 알린다(find_tools 로는 찾을 수 있다)", len(hidden)))
	}
	// **다 들어가면 다 알린다.** 고르는 것은 카탈로그가 클 때나 뜻이 있다.
	// 지역 도구 네 개짜리에 find_tools 만 쥐여 주면, 목표가 한국어라 Pick 이
	// 빈 값을 줄 때 **쓸 도구가 하나도 안 알려진다.**
	picked := visible
	if fitsAll := totalCost(visible); fitsAll > toolChars || l.DeferAll {
		picked = []mcpclient.Tool{findTools, toolHelp}
		if !l.DeferAll {
			picked = append(picked, mcpclient.Pick(visible, []string{goal}, toolChars)...)
		}
	}
	l.log("TOOLS", fmt.Sprintf("도구 %d개(보이는 것 %d개) 가운데 %d개를 알린다",
		len(l.Tools), len(visible), len(picked)))

	// 미루기로 돌리면 모델은 카탈로그의 낱말을 모른다 — 갈래만 짧게 보여 준다.
	overview := ""
	if l.DeferAll {
		overview = "\n\n" + mcpclient.Overview(visible, 14)
	}
	// 사람에게 하는 말과 기계에게 하는 말을 가른다. 도구 이름·설명이 영어면
	// 찾는 낱말도 영어라야 한다.
	system += "\n\n" + languageRule(toolsLookEnglish(l.Tools))

	if g := guideBlock(l.Skills, l.Tools); g != "" {
		system += g
		picked = append([]mcpclient.Tool{readReference}, picked...)
	}
	if l.Instructions != "" {
		system += "\n\n[도구 서버가 알려 준 쓰는 법]\n" + l.Instructions
	}
	if w := workflowBlock(l.Prompts); w != "" {
		picked = append([]mcpclient.Tool{getWorkflow}, picked...)
		system += "\n\n" + w
	}
	history := []*genai.Content{
		userTurn(system + "\n\n" + advertise(picked) + overview + "\n\n[목표]\n" + goal),
	}
	called := map[string]bool{}
	seenOut := map[string]string{} // 결과 -> 그 결과를 낸 도구

	for turn := 1; turn <= maxTurns; turn++ {
		history = l.compact(ctx, history)

		// **남은 예산을 알린다.** codex 는 rollout_budget 으로 같은 일을 한다 —
		// 모델이 얼마 남았는지 모르면 끝까지 탐색만 하다 끝난다.
		if left := maxTurns - turn; left <= 2 {
			history = append(history, userTurn(fmt.Sprintf(
				"[남은 턴 %d] 더 찾지 말고 지금까지 본 것으로 답해라.", left+1)))
		}

		text, err := l.say(ctx, history)
		if err != nil {
			return "", err
		}

		calls := agent.ParseToolCalls(text)
		if len(calls) == 0 {
			// **더 찾으라고 밀지 않는다.** CIE 가 40문항으로 쟀다 — 정답률
			// 52%→48%, 235초→420초, 도구 호출 1.9회→1.5회. 산문 턴만 하나 더
			// 늘었다. 여기서 다시 시도하지 마라.
			l.log("DONE", fmt.Sprintf("%d턴에서 도구를 그만 불렀다", turn))
			return text, nil
		}

		history = append(history, modelTurn(text))

		var results []string
		for _, c := range calls {
			key := c.Name + "|" + argKey(c.Args)
			if called[key] {
				results = append(results, toolResult(c.Name, "ERROR: 같은 호출을 되풀이했다 — 건너뛴다"))
				continue
			}
			called[key] = true

			out, err := l.dispatch(ctx, c.Name, c.Args)
			if err != nil {
				// **오류를 같은 대화로 되돌린다.** aider 의 reflected_message 와
				// 같은 자리다. 오류를 버리면 다음 턴이 같은 실수를 되풀이한다.
				//
				// 다만 **같은 오류가 또 나면 그 길은 막힌 것**이다. 실측에서
				// trace_native_values 가 같은 오류로 세 번 연속 불렸다 — 인자가
				// 조금씩 달라 호출 되풀이 막기에 안 걸렸다. 결과로 가른다.
				msg := "ERROR: " + err.Error()
				if prev, dup := seenOut[msg]; dup {
					l.log("TOOL_ERR", c.Name+": 같은 오류 되풀이 — 막힌 길이다")
					results = append(results, toolResult(c.Name,
						"ERROR: "+prev+" 에서 난 것과 **같은 오류**다. 이 길은 막혔다 — 다른 도구를 찾아라."))
					continue
				}
				seenOut[msg] = c.Name
				l.log("TOOL_ERR", c.Name+": "+err.Error())
				results = append(results, toolResult(c.Name, msg))
				continue
			}
			// **같은 결과를 두 번 싣지 않는다.** 창이 16k 라 되풀이는 비싸고,
			// 모델은 같은 것을 또 받으면 또 같은 것을 부른다. 실측에서
			// find_tools 가 같은 2,788자를 두 턴 이어 냈다.
			if prev, dup := seenOut[out]; dup {
				results = append(results, toolResult(c.Name,
					"ERROR: "+prev+" 에서 이미 받은 것과 같은 결과다 — 다른 도구를 불러라"))
				continue
			}
			seenOut[out] = c.Name

			clipped, cut := agent.ClipToTokens(out, resultTokens)
			if cut {
				clipped += "\n…(잘렸다)"
			}
			l.log("TOOL", fmt.Sprintf("%s → %d자", c.Name, len(out)))
			results = append(results, toolResult(c.Name, clipped))
		}
		history = append(history, userTurn(strings.Join(results, "\n")))
	}
	return "", fmt.Errorf("%d턴을 다 썼는데 끝나지 않았다", maxTurns)
}

// findTools 는 루프가 들고 있는 내장 도구다 — MCP 서버에 가지 않는다.
var findTools = mcpclient.Tool{
	Name:        "find_tools",
	Description: "쓸 수 있는 도구를 영어 낱말로 찾는다. 찾은 이름은 그대로 부를 수 있다.",
	Schema:      json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`),
}

// getWorkflow 는 서버가 알린 작업 절차를 받아 오는 내장 도구다.
var getWorkflow = mcpclient.Tool{
	Name:        "get_workflow",
	Description: "일 전체의 차례(workflow)를 받아 온다. 어떻게 할지 막히면 먼저 이것을 불러라.",
	Schema: json.RawMessage(`{"type":"object","properties":{` +
		`"name":{"type":"string"},` +
		`"arguments":{"type":"object","description":"그 절차가 요구하는 인자"}},` +
		`"required":["name"]}`),
}

// readReference 는 지침의 참고 문서를 필요할 때만 읽는 내장 도구다.
var readReference = mcpclient.Tool{
	Name:        "read_reference",
	Description: "쓰는 차례의 참고 문서를 읽는다. 어떤 도구를 어떤 차례로 쓸지 막히면 불러라.",
	Schema:      json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`),
}

// toolHelp 는 도구 하나의 입력을 묻는 내장 도구다.
var toolHelp = mcpclient.Tool{
	Name:        "tool_help",
	Description: "도구 하나가 받는 입력 이름을 묻는다. 인자를 모르면 지어내지 말고 이것을 불러라.",
	Schema:      json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`),
}

// workflowBlock 은 쓸 수 있는 절차를 짧게 알린다. 이름과 설명만이라 싸다.
func workflowBlock(ps []mcpclient.Prompt) string {
	if len(ps) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("[쓸 수 있는 작업 절차 — get_workflow 로 받아 온다]\n")
	for _, p := range ps {
		fmt.Fprintf(&b, "- %s: %s\n", p.Name, firstLineOf(p.Description))
		// **인자를 알려 주지 않으면 못 부른다.** 도구에서 겪은 것과 같다(#214).
		// 실측에서 절차 6개가 전부 「expected string, received undefined」 로
		// 떨어졌다.
		var req, opt []string
		for _, a := range p.Arguments {
			if a.Required {
				req = append(req, a.Name)
			} else {
				opt = append(opt, a.Name)
			}
		}
		if len(req) > 0 || len(opt) > 0 {
			line := "  인자:"
			if len(req) > 0 {
				line += " 필수 " + strings.Join(req, ", ")
			}
			if len(opt) > 0 {
				line += " / 그 밖에 " + strings.Join(opt, ", ")
			}
			b.WriteString(line + "\n")
		}
	}
	return b.String()
}

func firstLineOf(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 110 {
		s = s[:110] + "…"
	}
	return s
}

func (l *Loop) dispatch(ctx context.Context, name string, args map[string]any) (string, error) {
	if name == getWorkflow.Name {
		if l.GetPrompt == nil {
			return "", fmt.Errorf("작업 절차를 받아 올 길이 없다")
		}
		n, _ := args["name"].(string)
		inner, _ := args["arguments"].(map[string]any)
		body, err := l.GetPrompt(ctx, n, inner)
		if err != nil {
			return "", err
		}
		// **절차가 이름만 일러 주면 모델은 인자를 지어낸다.**
		//
		// 실측: 절차를 받아 open_binary·search_procedures 를 알았지만 스키마는
		// 못 봐서 target_path·query 로 불렀고, 고칠 때마다 턴을 하나씩 버려
		// 예산이 떨어졌다. 절차에 나온 도구의 입력을 **함께** 준다.
		return body + mcpclient.InputsMentionedIn(l.Tools, body), nil
	}
	if name == readReference.Name {
		n, _ := args["name"].(string)
		return l.readReference(n)
	}
	if name == toolHelp.Name {
		n, _ := args["name"].(string)
		if h := mcpclient.InputHint(l.Tools, n); h != "" {
			return h, nil
		}
		return "", fmt.Errorf("%s 라는 도구는 없다", n)
	}
	if name == findTools.Name {
		q, _ := args["query"].(string)
		return mcpclient.FindTools(l.Tools, q, 20), nil
	}
	return l.Call(ctx, name, args)
}

func (l *Loop) say(ctx context.Context, history []*genai.Content) (string, error) {
	it := l.LLM.GenerateContent(ctx, &model.LLMRequest{Contents: history}, false)
	var text string
	for resp, err := range it {
		if err != nil {
			return "", err
		}
		if resp == nil || resp.Content == nil {
			continue
		}
		for _, p := range resp.Content.Parts {
			text += p.Text
		}
	}
	return text, nil
}

func (l *Loop) log(kind, msg string) {
	if l.Log != nil {
		l.Log(kind, msg)
	}
}

// advertise 는 쓸 수 있는 도구를 알린다.
//
// 스키마를 `<tools>` 로 감싸지 **않는다.** 이 모델은 가까이 본 태그를 그대로
// 흉내 내서, 입력 태그 `<tools>` 를 제 출력에 쓴다(실측 3/3). 다른 글자로
// 알려 두면 `<tool_call>` 로 낼 확률이 오른다 — 그러면 vLLM 의 네이티브 파서도
// 맞아 든다. 아직 안 쟀다. 파서는 어느 쪽이든 받는다.
func advertise(tools []mcpclient.Tool) string {
	if len(tools) == 0 {
		return "[쓸 수 있는 도구가 없다 — 아는 것만으로 답해라]"
	}
	var b strings.Builder
	b.WriteString("[쓸 수 있는 도구]\n")
	for _, t := range tools {
		fmt.Fprintf(&b, "- %s: %s\n  입력: %s\n", t.Name, t.Description, string(t.Schema))
	}
	b.WriteString("\n부르려면 한 줄에 하나씩 이렇게 쓴다:\n" +
		`<tool_call>{"name":"도구이름","arguments":{...}}</tool_call>` +
		"\n더 부를 것이 없으면 도구를 쓰지 말고 답만 써라.")
	return b.String()
}

// toolsLookEnglish 는 도구 이름이 영어인지 본다 — 찾을 낱말을 어느 말로 쓸지
// 정하는 데 쓴다.
func toolsLookEnglish(ts []mcpclient.Tool) bool {
	for _, t := range ts {
		for _, r := range t.Name {
			if r > 127 {
				return false
			}
		}
	}
	return len(ts) > 0
}

// totalCost 는 도구를 다 알리는 데 드는 글자다.
func totalCost(ts []mcpclient.Tool) int {
	n := 0
	for _, t := range ts {
		n += len(t.Name) + len(t.Description) + len(t.Schema)
	}
	return n
}

func toolResult(name, body string) string {
	return fmt.Sprintf("<tool_result name=%q>\n%s\n</tool_result>", name, body)
}

func argKey(args map[string]any) string {
	b, _ := json.Marshal(args)
	return string(b)
}

func userTurn(s string) *genai.Content {
	return &genai.Content{Role: "user", Parts: []*genai.Part{{Text: s}}}
}

func modelTurn(s string) *genai.Content {
	return &genai.Content{Role: "model", Parts: []*genai.Part{{Text: s}}}
}

// fit 은 궤적을 창 안에 눌러 넣는다.
//
// keep 개는 절대 버리지 않는다 — 맨 앞(지시·도구·목표)과 요약이 거기 든다.
// 요약을 버리면 접은 뜻이 없어진다.
//
// 차례를 버려도 모자라면 **남은 글을 자른다.** 버리기만 하면 큰 차례 하나가
// 남아 예산을 넘긴 채로 모델에 가고, 그 요청은 통째로 떨어진다.
func fit(history []*genai.Content, budget, keep int) []*genai.Content {
	if len(history) <= keep || tokensOf(history) <= budget {
		return history
	}
	// 지켜야 하는 것 뒤에서, 짝을 지켜 오래된 것부터 버린다.
	out := append([]*genai.Content{}, history...)
	for len(out) > keep+2 && tokensOf(out) > budget {
		out = append(out[:keep], out[keep+2:]...)
	}
	if tokensOf(out) <= budget {
		return out
	}
	// 그래도 넘으면 차례마다 몫을 나눠 자른다.
	share := budget / len(out)
	if share < 64 {
		share = 64
	}
	for i, c := range out {
		for j, p := range c.Parts {
			if agent.EstimateTokens(p.Text) <= share {
				continue
			}
			cut, _ := agent.ClipToTokens(p.Text, share)
			out[i].Parts[j] = &genai.Part{Text: cut + "\n…(잘렸다)"}
		}
	}
	return out
}
