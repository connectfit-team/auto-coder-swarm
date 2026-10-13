// gatefix-eval 은 **관문 되먹임이 실제로 고칠 만한가**를 1분 안에 잰다.
//
// ACS 한 판은 10~20분이고 12판이면 서너 시간이다. 되먹임 글 한 줄을 고쳐 보려고
// 그걸 기다릴 수는 없다. 모형 저장소에 관문 위반을 심고, 도구를 쥔 고치기를
// 그대로 돌려, 관문이 지나는지 본다.
//
//	gatefix-eval                 # 다 돌린다
//	gatefix-eval -case duplicate # 하나만
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
	"github.com/connectfit-team/auto-coder-swarm/internal/agentloop"
	"github.com/connectfit-team/auto-coder-swarm/internal/llm"
	"github.com/connectfit-team/auto-coder-swarm/internal/orchestrator"
	"google.golang.org/adk/model"
)

// 관문이 바르게 잡았지만 모델이 못 고쳤던 것들이다(실측).
var cases = []struct {
	name string
	head map[string]string // HEAD 에 있던 것
	bad  map[string]string // 자식이 만든 것 (이 상태가 관문에 걸린다)
}{
	{
		name: "duplicate", // 중복 선언
		head: map[string]string{
			"proto/connect.proto": protoHead + "\nmessage ReceivedRequest {\n  string id = 1;\n}\n",
			"proto/service.proto": protoHead,
		},
		bad: map[string]string{
			"proto/service.proto": protoHead + "\nmessage ReceivedRequest {\n  bool held = 1;\n}\n",
		},
	},
	{
		name: "mixed-type", // 같은 것을 두 타입으로
		head: map[string]string{
			"proto/connect.proto": protoHead + "\nenum PendingStatus {\n  PENDING_STATUS_UNSPECIFIED = 0;\n}\n" +
				"\nmessage ReceivedRequest {\n  PendingStatus pending_status = 1;\n}\n",
		},
		bad: map[string]string{
			"proto/connect.proto": protoHead + "\nenum PendingStatus {\n  PENDING_STATUS_UNSPECIFIED = 0;\n}\n" +
				"\nmessage ReceivedRequest {\n  PendingStatus pending_status = 1;\n  int32 hold_status = 2;\n}\n",
		},
	},
	{
		name: "suffix-name", // 이 계약은 접두형인데 접미형으로 만들었다
		head: map[string]string{
			"proto/connect.proto": protoHead +
				"\nmessage RequestGetConnect {\n  string id = 1;\n}\n" +
				"\nmessage ResponseGetConnect {\n  string id = 1;\n}\n" +
				"\nmessage RequestListConnect {\n  string id = 1;\n}\n",
		},
		bad: map[string]string{
			"proto/connect.proto": protoHead +
				"\nmessage RequestGetConnect {\n  string id = 1;\n}\n" +
				"\nmessage ResponseGetConnect {\n  string id = 1;\n}\n" +
				"\nmessage RequestListConnect {\n  string id = 1;\n}\n" +
				"\nmessage UpdatePendingRequest {\n  string id = 1;\n}\n",
		},
	},
}

const protoHead = "syntax = \"proto3\";\n\npackage ceo.connect.ceoweb;\n"

func main() {
	only := flag.String("case", "", "하나만 돌린다")
	turns := flag.Int("turns", 6, "턴 예산")
	modelName := flag.String("model", envOr("LLM_MODEL_NAME", "gemma4:31b"), "모델")
	flag.Parse()

	ctx := context.Background()
	m := llm.FromEnv(*modelName, os.Getenv("RABBITMQ_URL"))

	ok, n := 0, 0
	for _, c := range cases {
		if *only != "" && c.name != *only {
			continue
		}
		n++
		start := time.Now()
		fixed, why := run(ctx, m, c.name, c.head, c.bad, *turns)
		mark := "✗"
		if fixed {
			mark = "✓"
			ok++
		}
		fmt.Printf("%s %-12s %s (%.0f초)\n", mark, c.name, why, time.Since(start).Seconds())
	}
	fmt.Printf("\n%d/%d\n", ok, n)
	if ok < n {
		os.Exit(1)
	}
}

func run(ctx context.Context, m model.LLM, name string,
	head, bad map[string]string, turns int) (bool, string) {

	repo, err := fixture(head, bad)
	if err != nil {
		return false, "모형을 못 만들었다: " + err.Error()
	}
	defer os.RemoveAll(repo)

	coder := agent.NewCoderAgent(m)
	check := func() string {
		diff := worktreeDiff(ctx, repo)
		if strings.TrimSpace(diff) == "" {
			return "아직 고친 것이 없다."
		}
		bad := orchestrator.CheckProtoChange(repo, diff)
		if len(bad) == 0 {
			return "관문을 지난다."
		}
		return "관문에 걸린다. 아래 자리만 고쳐라 — 처음부터 다시 쓰지 마라.\n\n" +
			orchestrator.AlignmentNote(bad)
	}
	edit := func(ctx context.Context, rel, how string) (string, error) {
		if _, err := coder.ModifyFile(ctx, filepath.Join(repo, rel), how); err != nil {
			return "", err
		}
		return "고쳤다. check 로 관문을 보라.", nil
	}

	tools, dispatch := orchestrator.RepoTools(repo, edit, check)
	first := check()
	if !strings.Contains(first, "걸린다") {
		return false, "모형이 관문에 안 걸린다 — 시험이 잘못됐다"
	}

	loop := &agentloop.Loop{
		LLM: m, Tools: tools, Call: dispatch, MaxTurns: turns,
		Log: func(kind, msg string) {
			if kind == "TOOL" || kind == "TOOL_ERR" {
				log.Printf("   %s %s", kind, msg)
			}
		},
	}
	system := "너는 계약(.proto)을 고치는 코더다.\n" +
		"지금 고친 것이 관문에 걸렸다. **걸린 자리만** 고쳐라 — 처음부터 다시 쓰지 마라.\n" +
		"고친 뒤에는 반드시 check 를 불러라."
	if _, err := loop.Run(ctx, system, first); err != nil {
		return false, "끝내지 못했다: " + err.Error()
	}
	if out := check(); strings.Contains(out, "지난다") {
		return true, "고쳤다"
	}
	return false, "아직 걸린다"
}

func fixture(head, bad map[string]string) (string, error) {
	dir, err := os.MkdirTemp("", "gatefix-*")
	if err != nil {
		return "", err
	}
	write := func(files map[string]string) error {
		for rel, body := range files {
			full := filepath.Join(dir, rel)
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
				return err
			}
		}
		return nil
	}
	if err := write(head); err != nil {
		return "", err
	}
	for _, a := range [][]string{
		{"init", "-q"}, {"add", "-A"},
		{"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "head"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, a...)...).CombinedOutput(); err != nil {
			return "", fmt.Errorf("git %v: %v %s", a, err, out)
		}
	}
	return dir, write(bad)
}

func worktreeDiff(ctx context.Context, repo string) string {
	out, err := exec.CommandContext(ctx, "git", "-C", repo, "diff", "HEAD").Output()
	if err != nil {
		return ""
	}
	return string(out)
}

func envOr(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}
