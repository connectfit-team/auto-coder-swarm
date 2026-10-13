package agent

import "testing"

// 실측으로 이 모델이 실제로 내놓은 글이다 — vLLM 의 tool_calls 는 빈 배열이었다.
// 3/3 모두 <tools> 였다.
func TestParsesWhatThisModelActuallyEmits(t *testing.T) {
	const observed = "<tools>\n" +
		`{"name": "read_code_file", "arguments": {"path": "internal/agent/coder.go"}}` + "\n</tools>"

	got := ParseToolCalls(observed)
	if len(got) != 1 {
		t.Fatalf("호출 1개여야 한다: %+v", got)
	}
	if got[0].Name != "read_code_file" {
		t.Errorf("이름: %q", got[0].Name)
	}
	if got[0].Args["path"] != "internal/agent/coder.go" {
		t.Errorf("인자: %+v", got[0].Args)
	}
}

func TestParsesTheOtherShapes(t *testing.T) {
	cases := map[string]string{
		"표준 태그":            `<tool_call>{"name":"list_files","arguments":{"path":"."}}</tool_call>`,
		"마크다운 펜스":          "```json\n{\"name\":\"list_files\",\"arguments\":{\"path\":\".\"}}\n```",
		"맨 JSON":           `{"name":"list_files","arguments":{"path":"."}}`,
		"parameters 로 쓴 것": `<tools>{"name":"list_files","parameters":{"path":"."}}</tools>`,
	}
	for what, text := range cases {
		got := ParseToolCalls(text)
		if len(got) != 1 || got[0].Name != "list_files" || got[0].Args["path"] != "." {
			t.Errorf("%s: %+v", what, got)
		}
	}
}

func TestParsesSeveralAndIgnoresProse(t *testing.T) {
	text := "먼저 파일을 읽겠다.\n" +
		`<tools>{"name":"read_code_file","arguments":{"path":"a.go"}}</tools>` + "\n" +
		`<tools>{"name":"read_code_file","arguments":{"path":"b.go"}}</tools>`
	if got := ParseToolCalls(text); len(got) != 2 {
		t.Errorf("둘을 다 잡아야 한다: %+v", got)
	}

	// 도구를 안 부른 턴을 부른 것으로 보면 루프가 안 끝난다.
	for _, text := range []string{
		"다 고쳤다. 더 부를 도구는 없다.",
		"<tools>이건 JSON 이 아니다</tools>",
		`{"arguments":{"path":"a.go"}}`,
	} {
		if got := ParseToolCalls(text); len(got) != 0 {
			t.Errorf("도구 호출이 아닌데 잡았다 (%q): %+v", text, got)
		}
	}
}

// 사내 형식(llm.BuildPrompt 가 이르는 꼴)도 받아야 한다.
func TestParsesInHouseFormat(t *testing.T) {
	text := `먼저 찾아본다. <|tool_call|>call:search_code{"query":"handOver"}<|tool_call|>`
	got := ParseToolCalls(text)
	if len(got) != 1 || got[0].Name != "search_code" || got[0].Args["query"] != "handOver" {
		t.Errorf("사내 형식을 못 읽었다: %+v", got)
	}
}
