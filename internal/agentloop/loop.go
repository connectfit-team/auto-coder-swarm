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

	MaxTurns     int // 0 이면 8
	ToolChars    int // 도구를 알리는 데 쓸 글자. 0 이면 창의 1/4
	ResultTokens int // 도구 결과 하나가 쓸 수 있는 토큰. 0 이면 1200

	// DeferAll 은 도구 스키마를 미리 알리지 않고 find_tools 로만 찾게 한다.
	// codex 가 검색 도구를 켰을 때 하는 것과 같다.
	DeferAll bool

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
		l.log("TOOLS", fmt.Sprintf("혼자 창을 먹는 도구 %d개는 숨긴다", len(hidden)))
	}
	picked := []mcpclient.Tool{findTools}
	if !l.DeferAll {
		picked = append(picked, mcpclient.Pick(visible, []string{goal}, toolChars)...)
	}
	l.log("TOOLS", fmt.Sprintf("도구 %d개(보이는 것 %d개) 가운데 %d개를 알린다",
		len(l.Tools), len(visible), len(picked)))

	history := []*genai.Content{
		userTurn(system + "\n\n" + advertise(picked) + "\n\n[목표]\n" + goal),
	}
	called := map[string]bool{}
	seenOut := map[string]string{} // 결과 -> 그 결과를 낸 도구

	for turn := 1; turn <= maxTurns; turn++ {
		history = trim(history, agent.InputTokenBudget())

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
				l.log("TOOL_ERR", c.Name+": "+err.Error())
				results = append(results, toolResult(c.Name, "ERROR: "+err.Error()))
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

func (l *Loop) dispatch(ctx context.Context, name string, args map[string]any) (string, error) {
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

// trim 은 궤적을 창 안에 눌러 넣는다.
//
// 첫 턴(지시·도구·목표)과 **가장 최근 것들**을 남긴다. 가운데가 먼저 빠진다 —
// 방금 본 도구 결과가 오래된 것보다 쓸모 있다.
func trim(history []*genai.Content, budget int) []*genai.Content {
	if len(history) <= 1 {
		return history
	}
	fits := func(h []*genai.Content) bool {
		n := 0
		for _, c := range h {
			for _, p := range c.Parts {
				n += agent.EstimateTokens(p.Text)
			}
		}
		return n <= budget
	}
	if fits(history) {
		return history
	}
	// **짝을 지켜 자른다.** 모델 턴과 그 결과는 한 묶음이라, 하나만 떼면
	// 부른 적 없는 결과나 결과 없는 호출이 남는다. codex 도 사용자 턴 경계로
	// 자른다(thread_rollout_truncation.rs).
	head, tail := history[:1], history[1:]
	for len(tail) > 2 {
		tail = tail[2:]
		if fits(append(append([]*genai.Content{}, head...), tail...)) {
			break
		}
	}
	return append(append([]*genai.Content{}, head...), tail...)
}
