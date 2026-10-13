package mcpclient

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
)

// fakeServer 는 MCP 서버 흉내다 — 줄 단위 JSON-RPC 로 답한다.
// 사이에 알림을 하나 끼워 보내, 내 id 가 아닌 줄을 넘기는지도 본다.
func fakeServer(t *testing.T) *Client {
	t.Helper()
	cin, sin := io.Pipe()   // 손 -> 서버
	sout, cout := io.Pipe() // 서버 -> 손

	go func() {
		r := bufio.NewReader(cin)
		for {
			line, err := r.ReadBytes('\n')
			if err != nil {
				return
			}
			var req struct {
				ID     int             `json:"id"`
				Method string          `json:"method"`
				Params json.RawMessage `json:"params"`
			}
			if json.Unmarshal(line, &req) != nil || req.Method == "notifications/initialized" {
				continue
			}
			// 묻지 않은 알림을 먼저 흘린다.
			fmt.Fprintln(cout, `{"jsonrpc":"2.0","method":"notifications/progress","params":{}}`)

			switch req.Method {
			case "initialize":
				fmt.Fprintf(cout, `{"jsonrpc":"2.0","id":%d,"result":{"serverInfo":{"name":"fake"},`+
					`"instructions":"이 서버는 이렇게 쓴다."}}`+"\n", req.ID)
			case "prompts/list":
				fmt.Fprintf(cout, `{"jsonrpc":"2.0","id":%d,"result":{"prompts":[`+
					`{"name":"trace_crash","description":"크래시를 증거와 잇는다"}]}}`+"\n", req.ID)
			case "prompts/get":
				fmt.Fprintf(cout, `{"jsonrpc":"2.0","id":%d,"result":{"description":"절차",`+
					`"messages":[{"role":"user","content":{"type":"text","text":"1. 심볼을 찾아라"}}]}}`+"\n", req.ID)
			case "tools/list":
				fmt.Fprintf(cout, `{"jsonrpc":"2.0","id":%d,"result":{"tools":[`+
					`{"name":"read_code_file","description":"Read a source file","inputSchema":{"type":"object"}},`+
					`{"name":"decompile","description":"Read one part of an app as code","inputSchema":{"type":"object"}}`+
					`]}}`+"\n", req.ID)
			case "tools/call":
				var p struct {
					Name string         `json:"name"`
					Args map[string]any `json:"arguments"`
				}
				json.Unmarshal(req.Params, &p)
				if p.Name == "boom" {
					fmt.Fprintf(cout, `{"jsonrpc":"2.0","id":%d,"result":{"isError":true,"content":[{"type":"text","text":"터졌다"}]}}`+"\n", req.ID)
					continue
				}
				fmt.Fprintf(cout, `{"jsonrpc":"2.0","id":%d,"result":{"content":[{"type":"text","text":%q}]}}`+"\n",
					req.ID, p.Name+":"+fmt.Sprint(p.Args["path"]))
			}
		}
	}()

	c := newPiped("fake", sin, sout)
	if err := c.handshake(context.Background()); err != nil {
		t.Fatalf("손을 못 맞췄다: %v", err)
	}
	return c
}

func TestHandshakeAndCall(t *testing.T) {
	c := fakeServer(t)

	if len(c.Tools()) != 2 {
		t.Fatalf("도구 2개여야 한다: %d", len(c.Tools()))
	}
	if c.Tools()[0].Name != "read_code_file" {
		t.Errorf("첫 도구: %q", c.Tools()[0].Name)
	}

	got, err := c.Call(context.Background(), "read_code_file", map[string]any{"path": "a.go"})
	if err != nil {
		t.Fatalf("부르지 못했다: %v", err)
	}
	if got != "read_code_file:a.go" {
		t.Errorf("결과: %q", got)
	}

	// 서버가 오류라고 하면 오류로 받는다 — 조용히 통과시키지 않는다.
	if _, err := c.Call(context.Background(), "boom", nil); err == nil {
		t.Error("오류를 오류로 안 받았다")
	}
}

// 창에 다 못 넣으니 추려야 한다.
func TestPickFitsTheWindow(t *testing.T) {
	big := Tool{Name: "compare_web_captures", Description: "compare captures",
		Schema: json.RawMessage(`{"` + strings.Repeat("x", 4000) + `":1}`)}
	small := Tool{Name: "read_code_file", Description: "Read a source file",
		Schema: json.RawMessage(`{"type":"object"}`)}
	other := Tool{Name: "goto_address", Description: "jump", Schema: json.RawMessage(`{}`)}

	got := Pick([]Tool{big, small, other}, []string{"read the code file"}, 500)
	if len(got) != 1 || got[0].Name != "read_code_file" {
		t.Fatalf("맞는 것 하나를 골라야 한다: %+v", names(got))
	}

	// 큰 것 하나 때문에 뒤의 작은 것까지 버리면 안 된다.
	got = Pick([]Tool{big, small}, []string{"compare", "read"}, 500)
	if len(got) != 1 || got[0].Name != "read_code_file" {
		t.Errorf("큰 것은 건너뛰고 작은 것을 넣어야 한다: %+v", names(got))
	}

	// 아무것도 안 맞으면 억지로 채우지 않는다.
	if got := Pick([]Tool{big, small, other}, []string{"firmware"}, 500); len(got) != 0 {
		t.Errorf("안 맞는 것을 채워 넣었다: %+v", names(got))
	}
}

func names(ts []Tool) []string {
	var out []string
	for _, t := range ts {
		out = append(out, t.Name)
	}
	return out
}

// 고를 말이 없으면 아무것도 주지 않는다 — 앞에서부터 채우면 「골랐다」 는
// 얼굴로 엉뚱한 것을 준다. 실측에서 한국어 목표가 그랬다.
func TestPickGivesNothingWhenNothingMatches(t *testing.T) {
	ts := []Tool{
		{Name: "current_document", Description: "the selected document", Schema: json.RawMessage(`{}`)},
		{Name: "capabilities", Description: "List provider capabilities", Schema: json.RawMessage(`{}`)},
	}
	if got := Pick(ts, []string{"분석 엔진이 무엇인지 알려 줘"}, 4000); len(got) != 0 {
		t.Errorf("못 골랐는데 골라 줬다: %+v", names(got))
	}
}

// 못 골랐을 때 모델이 스스로 찾을 길이 있어야 한다.
func TestFindTools(t *testing.T) {
	ts := []Tool{
		{Name: "current_document", Description: "the selected document", Schema: json.RawMessage(`{}`)},
		{Name: "capabilities", Description: "List provider capabilities and side effects", Schema: json.RawMessage(`{}`)},
	}
	got := FindTools(ts, "provider capabilities", 20)
	if !strings.Contains(got, "capabilities") {
		t.Errorf("찾지 못했다: %q", got)
	}
	if strings.Contains(got, `"type":"object"`) {
		t.Error("스키마까지 돌려주면 창을 먹는다")
	}
	if got := FindTools(ts, "없는낱말", 20); !strings.Contains(got, "영어 낱말로") {
		t.Errorf("찾을 말이 없을 때 일러 주지 않는다: %q", got)
	}
}

// 혼자서 창을 먹는 도구는 고르기 전에 숨긴다.
// REA 의 compare_web_captures 는 75,501자 — 우리 창의 두 배다.
func TestVisibleHidesOversizedTools(t *testing.T) {
	huge := Tool{Name: "compare_web_captures", Description: "compare",
		Schema: json.RawMessage(`{"` + strings.Repeat("x", MaxToolChars+100) + `":1}`)}
	small := Tool{Name: "capabilities", Description: "List provider capabilities",
		Schema: json.RawMessage(`{"type":"object"}`)}

	ok, hidden := Visible([]Tool{huge, small})
	if len(ok) != 1 || ok[0].Name != "capabilities" {
		t.Errorf("남은 것: %+v", names(ok))
	}
	if len(hidden) != 1 || hidden[0] != "compare_web_captures" {
		t.Errorf("숨긴 것: %v", hidden)
	}
}

// 찾은 도구를 **부를 수 있어야** 한다 — 이름만 주면 인자를 몰라 못 부른다.
// codex 도 검색 결과에 입력 속성 이름을 싣는다(mcp.rs build_mcp_search_text).
func TestFindToolsCarriesInputNames(t *testing.T) {
	ts := []Tool{{
		Name:        "decompile",
		Description: "Read one part of an app as code",
		Schema: json.RawMessage(`{"type":"object",
			"properties":{"procedure":{"type":"string"},"address":{"type":"string"}},
			"required":["procedure"]}`),
	}}
	got := FindTools(ts, "decompile", 20)
	for _, want := range []string{"입력: address, procedure", "필수: procedure"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q 가 없다:\n%s", want, got)
		}
	}
	// 전체 스키마까지 실으면 창을 먹는다 — codex 도 떼어 낸다.
	if strings.Contains(got, `"type":"object"`) {
		t.Errorf("스키마를 통째로 실었다:\n%s", got)
	}
}

// 미루기로 돌리면 모델은 카탈로그의 낱말을 모른다 — 갈래만 짧게 준다.
func TestOverviewIsSmallAndUseful(t *testing.T) {
	var ts []Tool
	for _, n := range []string{
		"open_binary", "open_jeb_project",
		"list_names", "list_procedures", "list_strings",
		"compare_web_captures",
	} {
		ts = append(ts, Tool{Name: n, Schema: json.RawMessage(`{}`)})
	}
	got := Overview(ts, 14)
	for _, want := range []string{"list(3)", "open(2)", "compare(1)"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q 가 없다: %s", want, got)
		}
	}
	// 많은 쪽이 앞에 와야 쓸모 있다.
	if strings.Index(got, "list(") > strings.Index(got, "open(") {
		t.Errorf("많은 갈래가 뒤에 있다: %s", got)
	}
	// 도구 이름처럼 보이면 안 된다 — 모델이 갈래를 그대로 불렀다.
	if !strings.Contains(got, "도구 이름이 아니다") {
		t.Errorf("이름이 아니라고 일러 주지 않는다: %s", got)
	}
	// 짧아야 한다 — 길면 창을 먹어 쓰는 뜻이 없다.
	if len(got) > 600 {
		t.Errorf("너무 길다: %d자", len(got))
	}
	if Overview(nil, 14) != "" {
		t.Error("도구가 없으면 빈 글이어야 한다")
	}
}

// 오류에는 까닭이 있어야 한다 — 「오류를 돌려줬다」 만으로는 못 고친다.
func TestCallErrorCarriesTheReason(t *testing.T) {
	c := fakeServer(t)
	_, err := c.Call(context.Background(), "boom", nil)
	if err == nil {
		t.Fatal("오류를 안 냈다")
	}
	if !strings.Contains(err.Error(), "터졌다") {
		t.Errorf("까닭이 없다: %v", err)
	}
}

// 서버가 주는 것을 버리지 않는다 — 쓰는 법과 작업 절차.
func TestHandshakeKeepsInstructionsAndPrompts(t *testing.T) {
	c := fakeServer(t)

	if c.Instructions() != "이 서버는 이렇게 쓴다." {
		t.Errorf("쓰는 법을 버렸다: %q", c.Instructions())
	}
	if len(c.Prompts()) != 1 || c.Prompts()[0].Name != "trace_crash" {
		t.Fatalf("작업 절차를 못 받았다: %+v", c.Prompts())
	}
	got, err := c.GetPrompt(context.Background(), "trace_crash", nil)
	if err != nil {
		t.Fatalf("절차를 못 받아 왔다: %v", err)
	}
	if !strings.Contains(got, "심볼을 찾아라") {
		t.Errorf("절차 내용이 없다: %q", got)
	}
}
