// agent 는 도구를 쥔 에이전트를 한 판 돌린다 — codex·Claude Code 와 같은 모양으로,
// 엔진은 우리 LLM 이다.
//
//	agent -config ~/.acs/mcp.json -goal "..."
//
// 설정은 codex·Claude Code 가 쓰는 그 형식이다(mcpServers).
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/connectfit-team/auto-coder-swarm/internal/agentloop"
	"github.com/connectfit-team/auto-coder-swarm/internal/llm"
	"github.com/connectfit-team/auto-coder-swarm/internal/mcpclient"
)

func main() {
	cfgPath := flag.String("config", os.Getenv("SWARM_MCP_CONFIG"), "MCP 설정 (mcpServers 형식)")
	goal := flag.String("goal", "", "시킬 일")
	system := flag.String("system", "너는 조사하는 에이전트다. 도구로 사실을 확인하고, 확인한 것만 말해라.", "지시")
	turns := flag.Int("turns", 8, "최대 턴")
	modelName := flag.String("model", envOr("LLM_MODEL_NAME", "gemma4:31b"), "모델 이름")
	listOnly := flag.Bool("list-tools", false, "붙은 도구만 보여 주고 끝낸다")
	deferAll := flag.Bool("defer-tools", true, "도구 스키마를 미리 알리지 않고 find_tools 로만 찾게 한다 (codex 와 같다)")
	flag.Parse()

	if *goal == "" && !*listOnly {
		log.Fatal("-goal 이 없다")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	conf, err := mcpclient.LoadConfig(*cfgPath)
	if err != nil {
		log.Fatalf("설정: %v", err)
	}
	clients, errs := mcpclient.StartAll(ctx, conf)
	for _, e := range errs {
		// 하나가 안 떠도 나머지로 간다.
		log.Printf("[MCP] 못 띄웠다: %v", e)
	}
	defer func() {
		for _, c := range clients {
			c.Close()
		}
	}()

	tools, owner := mcpclient.AllTools(clients)
	for _, c := range clients {
		log.Printf("[MCP] %s: 도구 %d개", c.Name(), len(c.Tools()))
	}

	if *listOnly {
		for _, t := range tools {
			fmt.Printf("%-44s %s\n", t.Name, firstLine(t.Description))
		}
		fmt.Printf("\n모두 %d개\n", len(tools))
		return
	}

	loop := &agentloop.Loop{
		LLM:      llm.FromEnv(*modelName, os.Getenv("RABBITMQ_URL")),
		Tools:    tools,
		MaxTurns: *turns,
		DeferAll: *deferAll,
		Call: func(ctx context.Context, name string, args map[string]any) (string, error) {
			c, ok := owner[name]
			if !ok {
				// 없는 도구를 지어낸 것이다 — 그대로 일러 주면 다음 턴에 고친다.
				return "", fmt.Errorf("%s 라는 도구는 없다", name)
			}
			return c.Call(ctx, name, args)
		},
		Log: func(kind, msg string) { log.Printf("[%s] %s", kind, msg) },
	}

	out, err := loop.Run(ctx, *system, *goal)
	if err != nil {
		log.Fatalf("끝내지 못했다: %v", err)
	}
	fmt.Println(out)
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 90 {
		s = s[:90] + "…"
	}
	return s
}

func envOr(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}
