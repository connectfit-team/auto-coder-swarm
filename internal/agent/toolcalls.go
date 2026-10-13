package agent

import (
	"encoding/json"
	"regexp"
	"strings"
)

// ToolCall 은 모델이 부르려는 도구 하나다.
type ToolCall struct {
	Name string
	Args map[string]any
}

// 모델이 쓰는 태그를 넓게 받는다. 어느 것이든 안쪽은 JSON 이다.
var toolTag = regexp.MustCompile(`(?s)<(tool_call|tools|function_call|tool)>(.*?)</(?:tool_call|tools|function_call|tool)>`)

var jsonFence = regexp.MustCompile("(?s)```(?:json)?\\s*(\\{.*?\\})\\s*```")

// 사내 형식 — llm.BuildPrompt 가 이 꼴로 쓰라고 이른다.
var inhouseCall = regexp.MustCompile(`(?s)<\|tool_call\|>\s*call:\s*(\w+)\s*(\{.*?\})\s*<\|tool_call\|>`)

// ParseToolCalls 는 모델이 낸 도구 호출을 긁어낸다.
//
// **네이티브 tool_calls 를 못 쓴다.** vLLM 은 `--tool-call-parser hermes` 로 떠
// 있어 `<tool_call>` 을 기다리는데, 이 모델(Qwen2.5-Coder-32B-AWQ)은 도구
// **목록**을 감싸는 입력 태그 `<tools>` 를 그대로 흉내 내서 낸다. 실측 3/3 이
// 그랬고, 그래서 응답의 tool_calls 는 늘 빈 배열이다.
//
// 태그를 넓게 받고 안쪽 JSON 만 본다. 태그를 안 쓰고 펜스나 맨 JSON 으로
// 내놓는 것도 받는다 — 모델이 형식을 흘리는 것은 흔하고, 그때마다 한 턴을
// 버리는 것이 훨씬 비싸다.
func ParseToolCalls(text string) []ToolCall {
	var out []ToolCall
	seen := map[string]bool{}

	add := func(raw string) {
		var c struct {
			Name string         `json:"name"`
			Args map[string]any `json:"arguments"`
			Alt  map[string]any `json:"parameters"`
		}
		if json.Unmarshal([]byte(strings.TrimSpace(raw)), &c) != nil || c.Name == "" {
			return
		}
		args := c.Args
		if args == nil {
			args = c.Alt
		}
		key := c.Name + "|" + raw
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, ToolCall{Name: c.Name, Args: args})
	}

	// 사내 형식을 먼저 본다 — 이름과 인자가 따로라 가장 또렷하다.
	for _, m := range inhouseCall.FindAllStringSubmatch(text, -1) {
		var args map[string]any
		if json.Unmarshal([]byte(m[2]), &args) != nil {
			continue
		}
		out = append(out, ToolCall{Name: m[1], Args: args})
	}
	if len(out) > 0 {
		return out
	}
	for _, m := range toolTag.FindAllStringSubmatch(text, -1) {
		add(m[2])
	}
	if len(out) > 0 {
		return out
	}
	for _, m := range jsonFence.FindAllStringSubmatch(text, -1) {
		add(m[1])
	}
	if len(out) > 0 {
		return out
	}
	// 태그도 펜스도 없으면 통째로 JSON 인지 본다.
	add(text)
	return out
}
