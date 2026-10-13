package agentloop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"strings"
	"testing"

	"google.golang.org/adk/model"
	"google.golang.org/genai"

	"github.com/connectfit-team/auto-coder-swarm/internal/mcpclient"
)

// fakeLLM 은 미리 적어 둔 답을 차례로 내놓는다. 받은 대화도 적어 둔다.
type fakeLLM struct {
	says []string
	n    int
	saw  [][]*genai.Content
}

func (f *fakeLLM) Name() string { return "fake" }

func (f *fakeLLM) GenerateContent(_ context.Context, req *model.LLMRequest, _ bool) iter.Seq2[*model.LLMResponse, error] {
	f.saw = append(f.saw, req.Contents)
	say := "다 했다."
	if f.n < len(f.says) {
		say = f.says[f.n]
	}
	f.n++
	return func(yield func(*model.LLMResponse, error) bool) {
		yield(&model.LLMResponse{Content: &genai.Content{Parts: []*genai.Part{{Text: say}}}}, nil)
	}
}

func tools() []mcpclient.Tool {
	return []mcpclient.Tool{
		{Name: "read_code_file", Description: "Read a source file", Schema: json.RawMessage(`{"type":"object"}`)},
	}
}

// 도구를 부르면 결과가 **같은 대화**로 되돌아가고, 그만 부르면 거기서 끝난다.
func TestLoopFeedsResultsBackAndStops(t *testing.T) {
	llm := &fakeLLM{says: []string{
		`<tools>{"name":"read_code_file","arguments":{"path":"a.go"}}</tools>`,
		"파일을 봤다. 끝.",
	}}
	var calls []string
	l := &Loop{
		LLM:   llm,
		Tools: tools(),
		Call: func(_ context.Context, name string, args map[string]any) (string, error) {
			calls = append(calls, name+":"+args["path"].(string))
			return "package main", nil
		},
	}

	out, err := l.Run(context.Background(), "너는 코더다", "a.go 를 읽어라")
	if err != nil {
		t.Fatalf("돌지 않았다: %v", err)
	}
	if out != "파일을 봤다. 끝." {
		t.Errorf("답: %q", out)
	}
	if len(calls) != 1 || calls[0] != "read_code_file:a.go" {
		t.Fatalf("도구를 안 불렀다: %v", calls)
	}
	// 둘째 턴이 도구 결과를 보고 있어야 한다 — 안 보면 쌓는 뜻이 없다.
	second := llm.saw[1]
	joined := ""
	for _, c := range second {
		for _, p := range c.Parts {
			joined += p.Text
		}
	}
	if !strings.Contains(joined, "package main") {
		t.Error("도구 결과가 다음 턴 대화에 없다")
	}
}

// 오류도 같은 대화로 되돌아가야 한다 — 버리면 다음 턴이 같은 실수를 한다.
func TestLoopReflectsToolErrors(t *testing.T) {
	llm := &fakeLLM{says: []string{
		`<tools>{"name":"read_code_file","arguments":{"path":"없다.go"}}</tools>`,
		"없는 파일이었다.",
	}}
	l := &Loop{
		LLM:   llm,
		Tools: tools(),
		Call: func(context.Context, string, map[string]any) (string, error) {
			return "", errNotFound{}
		},
	}
	if _, err := l.Run(context.Background(), "s", "g"); err != nil {
		t.Fatalf("오류 하나로 판을 접었다: %v", err)
	}
	joined := ""
	for _, c := range llm.saw[1] {
		for _, p := range c.Parts {
			joined += p.Text
		}
	}
	if !strings.Contains(joined, "그런 파일 없다") {
		t.Errorf("도구 오류가 대화에 안 들어갔다: %q", joined)
	}
}

type errNotFound struct{}

func (errNotFound) Error() string { return "그런 파일 없다" }

// 같은 호출을 되풀이하면 건너뛴다 — 안 그러면 루프가 제자리를 돈다.
func TestLoopSkipsRepeatedCalls(t *testing.T) {
	same := `<tools>{"name":"read_code_file","arguments":{"path":"a.go"}}</tools>`
	llm := &fakeLLM{says: []string{same, same, "끝."}}
	n := 0
	l := &Loop{
		LLM:   llm,
		Tools: tools(),
		Call: func(context.Context, string, map[string]any) (string, error) {
			n++
			return "x", nil
		},
	}
	if _, err := l.Run(context.Background(), "s", "g"); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("같은 호출을 %d번 실제로 불렀다", n)
	}
}

// 끝없이 돌지 않는다.
func TestLoopStopsAtBudget(t *testing.T) {
	forever := `<tools>{"name":"read_code_file","arguments":{"path":"a.go"}}</tools>`
	llm := &fakeLLM{says: []string{forever, forever, forever, forever, forever}}
	l := &Loop{
		LLM: llm, Tools: tools(), MaxTurns: 3,
		Call: func(_ context.Context, _ string, args map[string]any) (string, error) {
			return "x", nil
		},
	}
	// 인자를 바꿔 가며 부르게 해 되풀이 막기에 안 걸리게 한다.
	i := 0
	l.Call = func(context.Context, string, map[string]any) (string, error) {
		i++
		return "x", nil
	}
	if _, err := l.Run(context.Background(), "s", "g"); err == nil {
		t.Error("예산을 다 쓰고도 오류를 안 냈다")
	}
}

// 같은 결과를 두 번 싣지 않는다 — 창이 16k 라 되풀이는 비싸고, 같은 것을 또
// 받으면 모델은 또 같은 것을 부른다.
func TestLoopDoesNotRepeatIdenticalResults(t *testing.T) {
	llm := &fakeLLM{says: []string{
		`<tools>{"name":"read_code_file","arguments":{"path":"a.go"}}</tools>`,
		`<tools>{"name":"read_code_file","arguments":{"path":"b.go"}}</tools>`,
		"끝.",
	}}
	l := &Loop{
		LLM: llm, Tools: tools(),
		Call: func(context.Context, string, map[string]any) (string, error) {
			return "똑같은 내용", nil
		},
	}
	if _, err := l.Run(context.Background(), "s", "g"); err != nil {
		t.Fatal(err)
	}
	third := ""
	for _, c := range llm.saw[2] {
		for _, p := range c.Parts {
			third += p.Text
		}
	}
	if strings.Count(third, "똑같은 내용") != 1 {
		t.Errorf("같은 결과가 두 번 실렸다:\n%s", third)
	}
	if !strings.Contains(third, "이미 받은 것과 같은 결과다") {
		t.Error("같은 결과라고 일러 주지 않는다")
	}
}

// codex 처럼 — 검색 도구가 있으면 스키마를 미리 알리지 않는다.
// 골라 주는 것은 맞을 때만 이득이고, 틀리면 턴을 버린다.
func TestDeferAllAdvertisesOnlyFindTools(t *testing.T) {
	llm := &fakeLLM{says: []string{"끝."}}
	l := &Loop{LLM: llm, Tools: tools(), DeferAll: true,
		Call: func(context.Context, string, map[string]any) (string, error) { return "", nil }}
	if _, err := l.Run(context.Background(), "s", "read the code file"); err != nil {
		t.Fatal(err)
	}
	first := llm.saw[0][0].Parts[0].Text
	if !strings.Contains(first, "find_tools") {
		t.Error("find_tools 를 안 알렸다")
	}
	if strings.Contains(first, "read_code_file") {
		t.Error("미루기로 했는데 도구 스키마를 알렸다")
	}
}

// 예산이 다 되어 가면 모델에게 알린다 — 모르면 끝까지 찾기만 한다.
func TestLoopWarnsWhenBudgetRunsLow(t *testing.T) {
	call := `<tools>{"name":"read_code_file","arguments":{"path":"a.go"}}</tools>`
	llm := &fakeLLM{says: []string{call, call, call}}
	n := 0
	l := &Loop{LLM: llm, Tools: tools(), MaxTurns: 3,
		Call: func(context.Context, string, map[string]any) (string, error) {
			n++
			return fmt.Sprint("결과", n), nil
		}}
	l.Run(context.Background(), "s", "g")

	last := ""
	for _, c := range llm.saw[len(llm.saw)-1] {
		for _, p := range c.Parts {
			last += p.Text
		}
	}
	if !strings.Contains(last, "남은 턴") {
		t.Errorf("남은 예산을 안 알렸다:\n%s", last)
	}
}

// 서버가 알려 준 쓰는 법과 작업 절차를 버리지 않는다.
func TestLoopUsesServerInstructionsAndWorkflows(t *testing.T) {
	llm := &fakeLLM{says: []string{
		`<tools>{"name":"get_workflow","arguments":{"name":"trace_crash","arguments":{"symptom":"죽는다"}}}</tools>`,
		"절차대로 했다.",
	}}
	var asked string
	var gotArgs map[string]any
	l := &Loop{
		LLM: llm, Tools: tools(), DeferAll: true,
		Instructions: "REA 는 지역 산출물을 뜯는 도구다.",
		Prompts:      []mcpclient.Prompt{must()},
		GetPrompt: func(_ context.Context, n string, a map[string]any) (string, error) {
			asked = n
			gotArgs = a
			return "1. 심볼을 찾아라\n2. 호출을 따라가라", nil
		},
		Call: func(context.Context, string, map[string]any) (string, error) {
			return "", errors.New("MCP 도구를 부르면 안 된다")
		},
	}
	if _, err := l.Run(context.Background(), "너는 조사자다", "크래시를 조사해라"); err != nil {
		t.Fatal(err)
	}
	first := llm.saw[0][0].Parts[0].Text
	if !strings.Contains(first, "REA 는 지역 산출물을 뜯는 도구다.") {
		t.Error("서버가 알려 준 쓰는 법을 안 실었다")
	}
	if !strings.Contains(first, "trace_crash") || !strings.Contains(first, "get_workflow") {
		t.Error("작업 절차를 안 알렸다")
	}
	if asked != "trace_crash" {
		t.Errorf("절차를 안 받아 왔다: %q", asked)
	}
	second := flatten(llm.saw[1])
	if !strings.Contains(second, "심볼을 찾아라") {
		t.Error("받아 온 절차가 대화에 안 들어갔다")
	}
	// **인자를 넘겨야 받아 올 수 있다.** 실제 서버의 절차 6개가 전부 필수
	// 인자 때문에 떨어졌다.
	if gotArgs["symptom"] != "죽는다" {
		t.Errorf("절차 인자를 안 넘겼다: %+v", gotArgs)
	}
	if !strings.Contains(first, "필수 symptom") {
		t.Errorf("필수 인자를 안 알렸다:\n%s", first)
	}
}

// 필수 인자가 있는 절차.
func must() mcpclient.Prompt {
	var p mcpclient.Prompt
	p.Name = "trace_crash"
	p.Description = "크래시를 증거와 잇는다"
	p.Arguments = append(p.Arguments, struct {
		Name     string `json:"name"`
		Required bool   `json:"required"`
	}{Name: "symptom", Required: true})
	return p
}

// 같은 오류가 또 나면 그 길은 막힌 것이다 — 세 번 두드리게 두지 않는다.
// 실측에서 trace_native_values 가 같은 오류로 세 번 연속 불렸다(인자가 조금씩
// 달라 호출 되풀이 막기에 안 걸렸다).
func TestLoopStopsRepeatingTheSameFailure(t *testing.T) {
	calls := []string{
		`<tools>{"name":"read_code_file","arguments":{"path":"a.go"}}</tools>`,
		`<tools>{"name":"read_code_file","arguments":{"path":"b.go"}}</tools>`,
		"끝.",
	}
	llm := &fakeLLM{says: calls}
	n := 0
	l := &Loop{LLM: llm, Tools: tools(),
		Call: func(context.Context, string, map[string]any) (string, error) {
			n++
			return "", errors.New("씨앗 함수를 못 찾았다")
		}}
	if _, err := l.Run(context.Background(), "s", "g"); err != nil {
		t.Fatal(err)
	}
	third := flatten(llm.saw[2])
	if !strings.Contains(third, "이 길은 막혔다") {
		t.Errorf("같은 오류를 막힌 길이라고 안 했다:\n%s", third)
	}
	if strings.Count(third, "씨앗 함수를 못 찾았다") > 1 {
		t.Error("같은 오류를 두 번 실었다")
	}
}

// 도구가 몇 개뿐이면 다 알린다 — 고르는 것은 카탈로그가 클 때나 뜻이 있다.
// 지역 도구 네 개짜리에 find_tools 만 쥐여 주면 쓸 도구가 하나도 안 알려진다.
func TestSmallToolsetIsAdvertisedWhole(t *testing.T) {
	small := []mcpclient.Tool{
		{Name: "check", Description: "관문을 본다", Schema: json.RawMessage(`{"type":"object"}`)},
		{Name: "edit_file", Description: "파일을 고친다", Schema: json.RawMessage(`{"type":"object"}`)},
	}
	llm := &fakeLLM{says: []string{"끝."}}
	l := &Loop{LLM: llm, Tools: small,
		Call: func(context.Context, string, map[string]any) (string, error) { return "", nil }}

	// 목표가 한국어라 Pick 은 아무것도 못 고른다 — 그래도 다 알려야 한다.
	if _, err := l.Run(context.Background(), "s", "관문에 걸렸다. 고쳐라."); err != nil {
		t.Fatal(err)
	}
	first := llm.saw[0][0].Parts[0].Text
	for _, want := range []string{"check", "edit_file"} {
		if !strings.Contains(first, want) {
			t.Errorf("%q 를 안 알렸다:\n%s", want, first)
		}
	}
}

// **말이 안 되는 호출은 시도가 아니다.** 인자 이름이 틀리면 서버는 아무 일도
// 안 하는데 그 턴이 예산에서 빠진다 — 실측에서 한 판이 세 번 틀리고 성공한
// 호출 없이 끝났다.
func TestMalformedCallsDoNotBurnTheBudget(t *testing.T) {
	// 서로 **다른** 틀린 호출이어야 한다 — 똑같으면 되풀이 막기가 먼저 잡는다.
	bad1 := `<tools>{"name":"binary_overview","arguments":{"path":"/bin/true"}}</tools>`
	bad2 := `<tools>{"name":"binary_overview","arguments":{"target_path":"/bin/true"}}</tools>`
	good := `<tools>{"name":"read_code_file","arguments":{"path":"a.go"}}</tools>`
	llm := &fakeLLM{says: []string{bad1, bad2, good, "끝."}}

	n := 0
	l := &Loop{LLM: llm, Tools: tools(), MaxTurns: 3,
		Call: func(_ context.Context, name string, args map[string]any) (string, error) {
			n++
			if name == "binary_overview" {
				return "", errors.New(`Input validation error: Unrecognized key: "path"`)
			}
			return "package main", nil
		}}

	out, err := l.Run(context.Background(), "s", "g")
	if err != nil {
		t.Fatalf("세 턴짜리 예산인데 인자 틀리기로 다 썼다: %v", err)
	}
	if out != "끝." {
		t.Errorf("답: %q", out)
	}
	if n != 3 {
		t.Errorf("도구를 %d번 불렀다 — 세 번이어야 한다", n)
	}
}

// 되돌려주는 횟수는 묶는다 — 안 묶으면 못 고치는 판이 끝나지 않는다.
func TestFreeTurnsAreCapped(t *testing.T) {
	bad := `<tools>{"name":"binary_overview","arguments":{"path":"x"}}</tools>`
	llm := &fakeLLM{says: []string{bad, bad, bad, bad, bad, bad, bad, bad}}
	l := &Loop{LLM: llm, Tools: tools(), MaxTurns: 2,
		Call: func(context.Context, string, map[string]any) (string, error) {
			return "", errors.New("Input validation error")
		}}
	if _, err := l.Run(context.Background(), "s", "g"); err == nil {
		t.Error("끝없이 돌았다 — 예산이 묶이지 않는다")
	}
	if len(llm.saw) > 2+maxFreeTurns+1 {
		t.Errorf("턴을 %d번 썼다 — 묶이지 않는다", len(llm.saw))
	}
}
