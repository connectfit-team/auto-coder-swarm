package agentloop

import (
	"context"
	"encoding/json"
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
