package mcpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// Config 는 codex·Claude Code 가 쓰는 그 형식이다 — 설정을 그대로 옮겨 올 수 있게.
//
//	{"mcpServers": {"rea": {"command": "npx", "args": ["-y","rea-agents@latest","--mcp"]}}}
type Config struct {
	Servers map[string]ServerSpec `json:"mcpServers"`
}

type ServerSpec struct {
	Command  string            `json:"command"`
	Args     []string          `json:"args"`
	Env      map[string]string `json:"env"`
	Disabled bool              `json:"disabled"`
}

// LoadConfig 는 설정 파일을 읽는다. 없으면 빈 설정이다 — 도구 없이도 돌아야 한다.
func LoadConfig(path string) (Config, error) {
	if path == "" {
		return Config{}, nil
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	return c, nil
}

// StartAll 은 설정에 적힌 서버를 모두 띄운다.
//
// **하나가 안 떠도 나머지는 쓴다.** 도구 서버 하나 때문에 일을 통째로 못 하는
// 것은 손해다 — 못 뜬 것은 오류로 돌려주고 부르는 쪽이 기록한다.
func StartAll(ctx context.Context, c Config) ([]*Client, []error) {
	var names []string
	for n, s := range c.Servers {
		if !s.Disabled {
			names = append(names, n)
		}
	}
	sort.Strings(names) // 차례를 고정한다 — 도구 목록이 판마다 달라지면 못 잰다

	var out []*Client
	var errs []error
	for _, n := range names {
		s := c.Servers[n]
		argv := append([]string{s.Command}, s.Args...)
		env := os.Environ()
		for k, v := range s.Env {
			env = append(env, k+"="+v)
		}
		cl, err := Start(ctx, n, argv, env)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		out = append(out, cl)
	}
	return out, errs
}

// AllTools 는 붙은 서버의 도구를 한 묶음으로 준다.
// 이름이 겹치면 서버 이름을 앞에 붙여 가른다.
func AllTools(clients []*Client) ([]Tool, map[string]*Client) {
	seen := map[string]int{}
	for _, c := range clients {
		for _, t := range c.Tools() {
			seen[t.Name]++
		}
	}
	var all []Tool
	owner := map[string]*Client{}
	for _, c := range clients {
		for _, t := range c.Tools() {
			if seen[t.Name] > 1 {
				t.Name = c.Name() + "__" + t.Name
			}
			all = append(all, t)
			owner[t.Name] = c
		}
	}
	return all, owner
}

// AllPrompts 는 붙은 서버의 작업 절차를 한 묶음으로 준다.
func AllPrompts(clients []*Client) ([]Prompt, map[string]*Client) {
	var all []Prompt
	owner := map[string]*Client{}
	for _, c := range clients {
		for _, p := range c.Prompts() {
			if _, dup := owner[p.Name]; dup {
				p.Name = c.Name() + "__" + p.Name
			}
			all = append(all, p)
			owner[p.Name] = c
		}
	}
	return all, owner
}

// AllInstructions 는 서버들이 스스로 알려 준 쓰는 법을 모은다.
func AllInstructions(clients []*Client) string {
	var parts []string
	for _, c := range clients {
		if s := c.Instructions(); s != "" {
			parts = append(parts, "["+c.Name()+"] "+s)
		}
	}
	return strings.Join(parts, "\n")
}
