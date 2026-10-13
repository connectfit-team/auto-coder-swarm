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
		// **정당한 작업 + 중복.** 중복만 있으면 그것을 지웠을 때 diff 가 통째로
		// 비어, 「고친 것이 없다」 가 된다 — 실제 모양이 아니다. 자식은 필드를
		// 더하는 일을 하면서 메시지를 또 선언하는 실수를 한다.
		name: "duplicate",
		head: map[string]string{
			"proto/connect.proto": protoHead + "\nmessage ReceivedRequest {\n  string id = 1;\n}\n",
			"proto/service.proto": protoHead,
		},
		bad: map[string]string{
			"proto/connect.proto": protoHead +
				"\nmessage ReceivedRequest {\n  string id = 1;\n  bool held = 2;\n}\n",
			"proto/service.proto": protoHead + "\nmessage ReceivedRequest {\n  bool held = 1;\n}\n",
		},
	},
	{
		// 두 필드가 **같은 수정 안에** 있어야 관문이 본다 — 관문은 diff 를 읽는다.
		name: "mixed-type",
		head: map[string]string{
			"proto/connect.proto": protoHead + enumBlock + "\nmessage ReceivedRequest {\n  string id = 1;\n}\n",
		},
		bad: map[string]string{
			"proto/connect.proto": protoHead + enumBlock +
				"\nmessage ReceivedRequest {\n  string id = 1;\n" +
				"  PendingStatus pending_status = 2;\n  int32 hold_status = 3;\n}\n",
		},
	},
	{
		// 이름 버릇은 **스무 개 넘게** 봐야 센다(minNameSamples).
		name: "suffix-name",
		head: map[string]string{
			"proto/connect.proto": protoHead + prefixHabit(),
		},
		bad: map[string]string{
			"proto/connect.proto": protoHead + prefixHabit() +
				"\nmessage UpdatePendingRequest {\n  string id = 1;\n}\n",
		},
	},
	{
		// 있던 필드를 고치거나 지웠다 — 계약은 더하기만 한다.
		name: "edited-existing",
		head: map[string]string{
			"proto/connect.proto": protoHead +
				"\nmessage ReceivedRequest {\n  string id = 1;\n  string note = 2;\n}\n",
		},
		bad: map[string]string{
			"proto/connect.proto": protoHead +
				"\nmessage ReceivedRequest {\n  string id = 1;\n  bool note = 2;\n  bool held = 3;\n}\n",
		},
	},
	{
		// 속이 빈 메시지를 만들었다.
		name: "empty-message",
		head: map[string]string{
			"proto/connect.proto": protoHead + "\nmessage ReceivedRequest {\n  string id = 1;\n}\n",
		},
		bad: map[string]string{
			"proto/connect.proto": protoHead +
				"\nmessage ReceivedRequest {\n  string id = 1;\n  bool held = 2;\n}\n" +
				"\nmessage HoldStatusPayload {\n}\n",
		},
	},
	{
		// 이 파일에 이미 service 가 있는데 또 만들었다.
		name: "new-service",
		head: map[string]string{
			"proto/service.proto": protoHead +
				"\nmessage RequestGetConnect {\n  string id = 1;\n}\n" +
				"\nmessage ResponseGetConnect {\n  string id = 1;\n}\n" +
				"\nservice ConnectService {\n  rpc GetConnect(RequestGetConnect) returns (ResponseGetConnect);\n}\n",
		},
		bad: map[string]string{
			"proto/service.proto": protoHead +
				"\nmessage RequestGetConnect {\n  string id = 1;\n}\n" +
				"\nmessage ResponseGetConnect {\n  string id = 1;\n}\n" +
				"\nmessage RequestHoldConnect {\n  string id = 1;\n}\n" +
				"\nmessage ResponseHoldConnect {\n  bool ok = 1;\n}\n" +
				"\nservice ConnectService {\n  rpc GetConnect(RequestGetConnect) returns (ResponseGetConnect);\n}\n" +
				"\nservice HoldService {\n  rpc HoldConnect(RequestHoldConnect) returns (ResponseHoldConnect);\n}\n",
		},
	},
	{
		// 필드 번호가 겹친다.
		name: "number-collision",
		head: map[string]string{
			"proto/connect.proto": protoHead +
				"\nmessage ReceivedRequest {\n  string id = 1;\n  string note = 2;\n}\n",
		},
		bad: map[string]string{
			"proto/connect.proto": protoHead +
				"\nmessage ReceivedRequest {\n  string id = 1;\n  string note = 2;\n  bool held = 2;\n}\n",
		},
	},
	{
		// enum 의 0 번을 뜻 있는 값으로 썼다 — 값이 없을 때의 기본값이라
		// 여태 있던 것이 모두 그 값으로 읽힌다.
		name: "enum-zero",
		head: map[string]string{
			"proto/connect.proto": protoHead + "\nmessage ReceivedRequest {\n  string id = 1;\n}\n",
		},
		bad: map[string]string{
			"proto/connect.proto": protoHead +
				"\nenum HoldStatus {\n  HOLD_STATUS_HELD = 0;\n  HOLD_STATUS_RELEASED = 1;\n}\n" +
				"\nmessage ReceivedRequest {\n  string id = 1;\n  HoldStatus hold_status = 2;\n}\n",
		},
	},
	{
		// 담을 자리만 만들고 바꾸는 길(rpc)을 안 냈다 — 실측에서 마지막까지
		// 남은 실패 갈래다.
		name: "state-no-rpc",
		head: map[string]string{
			"proto/connect.proto": protoHead + prefixHabit() +
				"\nmessage ReceivedRequest {\n  string id = 1;\n}\n" +
				"\nservice ConnectService {\n  rpc RequestOp1Call(RequestOp1) returns (ResponseOp1);\n}\n",
		},
		bad: map[string]string{
			"proto/connect.proto": protoHead + prefixHabit() +
				"\nmessage ReceivedRequest {\n  string id = 1;\n  PendingStatus pending_status = 2;\n}\n" +
				"\nenum PendingStatus {\n  PENDING_STATUS_UNSPECIFIED = 0;\n  PENDING_STATUS_HELD = 1;\n}\n" +
				"\nservice ConnectService {\n  rpc RequestOp1Call(RequestOp1) returns (ResponseOp1);\n}\n",
		},
	},
}

const protoHead = "syntax = \"proto3\";\n\npackage ceo.connect.ceoweb;\n"

const enumBlock = "\nenum PendingStatus {\n  PENDING_STATUS_UNSPECIFIED = 0;\n  PENDING_STATUS_HELD = 1;\n}\n"

// prefixHabit 은 이 계약이 접두형을 쓴다는 것을 관문이 셀 만큼 만든다.
func prefixHabit() string {
	var b strings.Builder
	for i := 1; i <= 13; i++ {
		fmt.Fprintf(&b, "\nmessage RequestOp%d {\n  string id = 1;\n}\n", i)
		fmt.Fprintf(&b, "\nmessage ResponseOp%d {\n  string id = 1;\n}\n", i)
	}
	return b.String()
}

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
	// **운영과 같게 HEAD 를 기준으로 준다.** 안 주면 「이번에 만든 것」 을 지우는
	// 것도 「원래 있던 이름이 사라졌다」 가 된다 — 모형이 운영 배선을 안 따라
	// 하면 없는 결함을 만든다. 실제로 이것 때문에 duplicate 가 헛돌았다.
	coder.SetBaseline(func(path string) (string, bool) {
		rel, err := filepath.Rel(repo, path)
		if err != nil {
			return "", false
		}
		out, err := exec.Command("git", "-C", repo, "show", "HEAD:"+rel).Output()
		if err != nil {
			return "", false
		}
		return string(out), true
	})
	check := func() string {
		diff := worktreeDiff(ctx, repo)
		if strings.TrimSpace(diff) == "" {
			return "아직 고친 것이 없다."
		}
		var notes []string
		if bad := orchestrator.CheckProtoChange(repo, diff); len(bad) > 0 {
			notes = append(notes, orchestrator.AlignmentNote(bad))
		}
		// handOver 가 t.req.AddsState 일 때 따로 부르는 관문이다 — 운영과 같은
		// 것을 보려면 여기서도 불러야 한다.
		if bad := orchestrator.CheckStateNeedsRPC(repo, diff); len(bad) > 0 {
			notes = append(notes, orchestrator.AlignmentNote(bad))
		}
		return orchestrator.GateReport(diff, notes)
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
		Verify: func() (bool, string) {
			out := check()
			return strings.Contains(out, "지난다"), out
		},
		Log: func(kind, msg string) {
			if strings.HasPrefix(kind, "TOOL") || kind == "NOT_DONE" {
				log.Printf("   %s %s", kind, msg)
			}
		},
	}
	system := "너는 계약(.proto)을 고치는 코더다.\n" +
		"지금 고친 것이 관문에 걸렸다. **걸린 자리만** 고쳐라 — 처음부터 다시 쓰지 마라.\n" +
		"고친 뒤에는 반드시 check 를 불러라."
	answer, err := loop.Run(ctx, system, first)
	if err != nil {
		return false, "끝내지 못했다: " + err.Error()
	}
	if out := check(); strings.Contains(out, "지난다") {
		return true, "고쳤다"
	} else {
		// **틀렸으면 무엇이 남았는지 보여 준다.** 모르면 못 고친다(#233).
		fmt.Printf("   남은 관문: %s\n", clip(out, 300))
		fmt.Printf("   마지막 답: %s\n", clip(answer, 300))
		fmt.Printf("   지금 diff: %s\n", clip(worktreeDiff(ctx, repo), 400))
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

func clip(s string, n int) string {
	r := []rune(strings.ReplaceAll(strings.TrimSpace(s), "\n", " ⏎ "))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + "…"
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
