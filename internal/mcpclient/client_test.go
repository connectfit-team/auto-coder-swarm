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
				fmt.Fprintf(cout, `{"jsonrpc":"2.0","id":%d,"result":{"serverInfo":{"name":"fake"}}}`+"\n", req.ID)
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
