// Package mcpclient 는 MCP stdio 서버에 붙어 도구를 부른다.
//
// 서버는 자식 프로세스로 띄우고 줄 단위 JSON-RPC 로 말한다. 사내에 쓰던 것이
// 없어 직접 쓴다 — 쓰는 것은 initialize·tools/list·tools/call 셋뿐이다.
package mcpclient

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Tool 은 서버가 알린 도구 하나다.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Schema      json.RawMessage `json:"inputSchema"`
}

// Prompt 는 서버가 알린 작업 절차 하나다.
type Prompt struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Arguments   []struct {
		Name     string `json:"name"`
		Required bool   `json:"required"`
	} `json:"arguments"`
}

type Client struct {
	name         string
	cmd          *exec.Cmd
	in           io.WriteCloser
	out          *bufio.Reader
	tools        []Tool
	prompts      []Prompt
	instructions string

	mu  sync.Mutex
	seq int
}

type rpcReq struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcResp struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// Start 는 서버를 띄우고 손을 맞춘 뒤 도구 목록까지 받아 둔다.
func Start(ctx context.Context, name string, argv []string, env []string) (*Client, error) {
	if len(argv) == 0 {
		return nil, fmt.Errorf("%s: 실행할 명령이 없다", name)
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	if len(env) > 0 {
		cmd.Env = env
	}
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	// 서버의 stderr 는 버린다 — 진단은 돌려받는 오류로 한다.
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("%s: 띄우지 못했다: %w", name, err)
	}

	c := &Client{name: name, cmd: cmd, in: in, out: bufio.NewReaderSize(out, 1<<20)}
	if err := c.handshake(ctx); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

// newPiped 는 자식 프로세스 없이 붙인다 — 시험에서 쓴다.
func newPiped(name string, in io.WriteCloser, out io.Reader) *Client {
	return &Client{name: name, in: in, out: bufio.NewReaderSize(out, 1<<20)}
}

// handshake 는 손을 맞추고 도구 목록까지 받아 둔다.
func (c *Client) handshake(ctx context.Context) error {
	init, err := c.call(ctx, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "auto-coder-swarm", "version": "1"},
	})
	if err != nil {
		return err
	}
	var hello struct {
		Instructions string `json:"instructions"`
	}
	if json.Unmarshal(init, &hello) == nil {
		c.instructions = strings.TrimSpace(hello.Instructions)
	}
	// 알림에는 id 가 없고 답도 없다.
	if err := c.write(rpcReq{JSONRPC: "2.0", Method: "notifications/initialized", Params: map[string]any{}}); err != nil {
		return err
	}
	raw, err := c.call(ctx, "tools/list", map[string]any{})
	if err != nil {
		return err
	}
	var listed struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.Unmarshal(raw, &listed); err != nil {
		return fmt.Errorf("%s: 도구 목록을 못 읽었다: %w", c.name, err)
	}
	c.tools = listed.Tools

	// 절차는 없을 수도 있다 — 없다고 손을 못 맞춘 것은 아니다.
	if raw, err := c.call(ctx, "prompts/list", map[string]any{}); err == nil {
		var ps struct {
			Prompts []Prompt `json:"prompts"`
		}
		if json.Unmarshal(raw, &ps) == nil {
			c.prompts = ps.Prompts
		}
	}
	return nil
}

func (c *Client) Name() string  { return c.name }
func (c *Client) Tools() []Tool { return c.tools }

// Instructions 는 서버가 스스로 알려 주는 쓰는 법이다.
// 이것을 버리면 서버가 아는 것을 모델이 모른 채 더듬는다.
func (c *Client) Prompts() []Prompt    { return c.prompts }
func (c *Client) Instructions() string { return c.instructions }

// GetPrompt 는 작업 절차 하나를 글로 받는다.
func (c *Client) GetPrompt(ctx context.Context, name string, args map[string]any) (string, error) {
	raw, err := c.call(ctx, "prompts/get", map[string]any{"name": name, "arguments": args})
	if err != nil {
		return "", err
	}
	var res struct {
		Description string `json:"description"`
		Messages    []struct {
			Role    string `json:"role"`
			Content struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return "", fmt.Errorf("%s/%s: 절차를 못 읽었다: %w", c.name, name, err)
	}
	var b strings.Builder
	if res.Description != "" {
		b.WriteString(res.Description + "\n\n")
	}
	for _, m := range res.Messages {
		if m.Content.Type == "text" {
			b.WriteString(m.Content.Text + "\n")
		}
	}
	return b.String(), nil
}

// Call 은 도구 하나를 부르고 글로 된 결과를 돌려준다.
func (c *Client) Call(ctx context.Context, tool string, args map[string]any) (string, error) {
	raw, err := c.call(ctx, "tools/call", map[string]any{"name": tool, "arguments": args})
	if err != nil {
		return "", err
	}
	var res struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return "", fmt.Errorf("%s/%s: 결과를 못 읽었다: %w", c.name, tool, err)
	}
	text := ""
	for _, p := range res.Content {
		if p.Type == "text" {
			text += p.Text
		}
	}
	if res.IsError {
		// **까닭을 함께 돌려준다.** 「오류를 돌려줬다」 만으로는 고칠 수 없다 —
		// 모델에게도, 로그를 보는 사람에게도.
		return text, fmt.Errorf("%s/%s: %s", c.name, tool, oneLine(text, 400))
	}
	return text, nil
}

func (c *Client) Close() error {
	if c.in != nil {
		c.in.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		// 곱게 끝나기를 잠깐 기다리고, 안 되면 끊는다.
		done := make(chan struct{})
		go func() { c.cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			c.cmd.Process.Kill()
		}
	}
	return nil
}

func (c *Client) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	c.seq++
	id := c.seq
	c.mu.Unlock()

	if err := c.write(rpcReq{JSONRPC: "2.0", ID: id, Method: method, Params: params}); err != nil {
		return nil, err
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		line, err := c.out.ReadBytes('\n')
		if err != nil {
			return nil, fmt.Errorf("%s/%s: 답이 끊겼다: %w", c.name, method, err)
		}
		var r rpcResp
		// 서버가 사이에 알림을 끼워 보낼 수 있다 — 내 id 가 아니면 넘긴다.
		if json.Unmarshal(line, &r) != nil || r.ID != id {
			continue
		}
		if r.Error != nil {
			return nil, fmt.Errorf("%s/%s: %s", c.name, method, r.Error.Message)
		}
		return r.Result, nil
	}
}

func (c *Client) write(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = c.in.Write(append(b, '\n'))
	return err
}

// oneLine 은 오류를 로그 한 줄에 들어가게 줄인다.
func oneLine(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > max {
		s = s[:max] + "…"
	}
	if s == "" {
		return "(까닭을 안 돌려줬다)"
	}
	return s
}
