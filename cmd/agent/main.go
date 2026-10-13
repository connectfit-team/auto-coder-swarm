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
	"path/filepath"
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
	verifyOnly := flag.Bool("verify", false, "서버가 내놓는 것을 다 받고 있는지 전수로 본다 (LLM 안 씀)")
	evalPath := flag.String("eval", "", "답을 아는 물음들을 풀게 하고 점수를 낸다")
	skillDir := flag.String("skills", defaultSkillDir(), "도구 쓰는 차례가 깔린 곳 (SKILL.md)")
	deferAll := flag.Bool("defer-tools", true, "도구 스키마를 미리 알리지 않고 find_tools 로만 찾게 한다 (codex 와 같다)")
	flag.Parse()

	if *goal == "" && !*listOnly && !*verifyOnly && *evalPath == "" {
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
	prompts, promptOwner := mcpclient.AllPrompts(clients)
	for _, c := range clients {
		log.Printf("[MCP] %s: 도구 %d개 · 절차 %d개 · 쓰는 법 %d자",
			c.Name(), len(c.Tools()), len(c.Prompts()), len(c.Instructions()))
	}

	if *verifyOnly {
		if verify(ctx, clients) != 0 {
			os.Exit(1)
		}
		return
	}

	if *listOnly {
		for _, t := range tools {
			fmt.Printf("%-44s %s\n", t.Name, firstLine(t.Description))
		}
		fmt.Printf("\n모두 %d개\n", len(tools))
		return
	}

	skills := agentloop.LoadSkills(*skillDir)
	for _, s := range skills {
		log.Printf("[SKILL] %s (참고 문서 %d개)", s.Name, len(s.Refs))
	}

	newLoop := func(goal string, turns int) *agentloop.Loop {
		return &agentloop.Loop{
			LLM:          llm.FromEnv(*modelName, os.Getenv("RABBITMQ_URL")),
			Tools:        tools,
			MaxTurns:     turns,
			DeferAll:     *deferAll,
			Instructions: mcpclient.AllInstructions(clients),
			Skills:       skills,
			Prompts:      prompts,
			GetPrompt: func(ctx context.Context, name string, args map[string]any) (string, error) {
				c, ok := promptOwner[name]
				if !ok {
					return "", fmt.Errorf("%s 라는 절차는 없다", name)
				}
				return c.GetPrompt(ctx, name, args)
			},
			Call: callTool(tools, owner),
			Log:  func(kind, msg string) { log.Printf("[%s] %s", kind, msg) },
		}
	}

	if *evalPath != "" {
		os.Exit(runEval(ctx, *evalPath, newLoop, *system))
	}

	loop := &agentloop.Loop{
		LLM:          llm.FromEnv(*modelName, os.Getenv("RABBITMQ_URL")),
		Tools:        tools,
		MaxTurns:     *turns,
		DeferAll:     *deferAll,
		Instructions: mcpclient.AllInstructions(clients),
		Prompts:      prompts,
		GetPrompt: func(ctx context.Context, name string, args map[string]any) (string, error) {
			c, ok := promptOwner[name]
			if !ok {
				return "", fmt.Errorf("%s 라는 절차는 없다", name)
			}
			return c.GetPrompt(ctx, name, args)
		},
		Call: callTool(tools, owner),
		Log:  func(kind, msg string) { log.Printf("[%s] %s", kind, msg) },
	}

	out, err := loop.Run(ctx, *system, *goal)
	if err != nil {
		log.Fatalf("끝내지 못했다: %v", err)
	}
	fmt.Println(out)
}

// callTool 은 도구를 부르고, 틀렸을 때 **고칠 단서를 함께** 돌려준다.
func callTool(tools []mcpclient.Tool, owner map[string]*mcpclient.Client) agentloop.Dispatch {
	return func(ctx context.Context, name string, args map[string]any) (string, error) {
		c, ok := owner[name]
		if !ok {
			// 「없다」 고만 하면 모델은 또 지어내거나 포기한다. 실측에서 목표
			// 문구를 보고 capabilities 를 지어냈는데, 그건 CLI 명령이었다.
			return "", fmt.Errorf("%s 라는 도구는 없다. 비슷한 것:\n%s",
				name, mcpclient.FindTools(tools, name, 5))
		}
		out, err := c.Call(ctx, name, args)
		if err != nil {
			// **인자 이름을 지어냈을 때 맞는 이름을 준다.** 서버는 「Unrecognized
			// key」 라고만 하고, 그것만으로는 또 지어낸다.
			if hint := mcpclient.InputHint(tools, name); hint != "" {
				return out, fmt.Errorf("%w\n%s", err, hint)
			}
		}
		return out, err
	}
}

// defaultSkillDir 은 REA 가 `rea setup` 으로 지침을 까는 곳이다.
func defaultSkillDir() string {
	if v := strings.TrimSpace(os.Getenv("SWARM_SKILL_DIR")); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".agents", "skills")
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
