// reviewbench 는 **검토자·비평가가 실제로 맞히는지** 잰다.
//
// 오늘 두 번, 나쁜 수정을 둘 다 통과시켰다 — 없는 RPC 를 부르는 것과
// 요청과 상관없는 한 줄짜리. 그런데 그 정확도를 한 번도 잰 적이 없어서
// 무엇을 고쳐야 하는지도 몰랐다. 판이 없으면 고칠 수 없다.
//
//	go run ./cmd/reviewbench            # 기본 모델
//	go run ./cmd/reviewbench -model X   # 모델을 바꿔 견준다
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
	"github.com/connectfit-team/auto-coder-swarm/internal/llm"
)

type reviewCase struct {
	Name     string `json:"name"`
	Request  string `json:"request"`
	Analysis string `json:"analysis"`
	Diff     string `json:"diff"`
	Want     string `json:"want"` // block | pass
	Why      string `json:"why"`
}

func main() {
	model := flag.String("model", envOr("BENCH_MODEL", "gemma4:latest"), "잴 모델")
	dir := flag.String("cases", "internal/agent/testdata/review_cases", "문항 폴더")
	amqp := flag.String("amqp", envOr("SWARM_AMQP", "amqp://guest:guest@192.168.120.54:5672/"), "모델 버스")
	timeout := flag.Duration("timeout", 5*time.Minute, "문항 하나에 줄 시간")
	flag.Parse()

	cases, err := load(*dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "문항을 못 읽었다:", err)
		os.Exit(1)
	}

	m := llm.FromEnv(*model, *amqp)
	reviewer := agent.NewReviewerAgent(m)
	critic := agent.NewCriticAgent(m)

	var blockOK, blockAll, passOK, passAll int
	fmt.Printf("모델 %s · 문항 %d개\n\n", *model, len(cases))

	for _, c := range cases {
		ctx, cancel := context.WithTimeout(context.Background(), *timeout)
		input := fmt.Sprintf("DIFF:\n%s\n\nPRE-BENCH:\n\nPOST-BENCH:\n", clip(c.Diff, 6000))

		blocked, why := judge(ctx, reviewer, critic, input, c)
		cancel()

		want := c.Want == "block"
		hit := blocked == want
		mark := "✗"
		if hit {
			mark = "✓"
		}
		if want {
			blockAll++
			if hit {
				blockOK++
			}
		} else {
			passAll++
			if hit {
				passOK++
			}
		}
		fmt.Printf("%s %-28s 바람=%s 얻음=%s  %s\n", mark, c.Name, c.Want, verdictWord(blocked), clip(why, 110))
	}

	fmt.Printf("\n막아야 할 것 %d/%d · 통과시켜야 할 것 %d/%d\n", blockOK, blockAll, passOK, passAll)
	if blockAll+passAll > 0 {
		fmt.Printf("합계 %d/%d\n", blockOK+passOK, blockAll+passAll)
	}
}

// judge 는 실제 흐름과 같은 순서로 본다 — 비평가가 먼저, 그다음 검토자.
func judge(ctx context.Context, r *agent.ReviewerAgent, c *agent.CriticAgent, input string, rc reviewCase) (bool, string) {
	if resp, err := c.Process(ctx, input); err == nil && strings.TrimSpace(resp) != "" {
		if cv := agent.ParseCriticVerdict(resp, rc.Diff); cv.Blocking {
			return true, "비평가: " + cv.Why
		}
	}
	resp, err := r.ProcessWithContext(ctx, input, rc.Request, rc.Analysis)
	if err != nil {
		return false, "검토자 호출 실패: " + err.Error()
	}
	if strings.TrimSpace(resp) == "" {
		return false, "검토자가 빈 응답을 냈다"
	}
	rv := agent.ParseReviewerVerdict(resp, rc.Diff)
	return rv.Blocking, "검토자: " + rv.Why
}

func load(dir string) ([]reviewCase, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	var out []reviewCase
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var c reviewCase
		if err := json.Unmarshal(b, &c); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		out = append(out, c)
	}
	return out, nil
}

func verdictWord(b bool) string {
	if b {
		return "block"
	}
	return "pass"
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + " …"
}
