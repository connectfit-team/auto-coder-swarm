package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/connectfit-team/auto-coder-swarm/internal/agentloop"
)

// Question 은 답을 아는 물음 하나다.
//
// 정답을 **글자로** 적는다. 모델이 어떻게 풀든 상관없고, 답에 그 글자가 있으면
// 맞은 것이다. 푸는 길을 적으면 길이 바뀔 때마다 채점기를 고쳐야 한다.
type Question struct {
	Name   string   `json:"name"`
	Goal   string   `json:"goal"`
	Expect []string `json:"expect"`     // 다 들어 있어야 한다
	Any    []string `json:"expect_any"` // 이 가운데 **하나라도** 있으면 된다
	Forbid []string `json:"forbid"`     // 하나라도 있으면 틀린 것이다
	Turns  int      `json:"turns"`
}

type result struct {
	q       Question
	ok      bool
	why     string
	seconds float64
	tools   int
}

// runEval 은 물음을 차례로 풀게 하고 점수를 낸다.
//
// **정확도를 못 재면 올릴 수도 없다.** 상용 에이전트와 견주려면 같은 물음에
// 같은 잣대를 써야 한다.
func runEval(ctx context.Context, path string, mk func(goal string, turns int) *agentloop.Loop, system string) int {
	raw, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "물음을 못 읽었다:", err)
		return 2
	}
	var qs []Question
	if err := json.Unmarshal(raw, &qs); err != nil {
		fmt.Fprintln(os.Stderr, "물음이 JSON 이 아니다:", err)
		return 2
	}

	var rs []result
	for i, q := range qs {
		fmt.Printf("\n[%d/%d] %s\n", i+1, len(qs), q.Name)
		turns := q.Turns
		if turns <= 0 {
			turns = 8
		}
		loop := mk(q.Goal, turns)
		used := 0
		inner := loop.Log
		loop.Log = func(kind, msg string) {
			if kind == "TOOL" {
				used++
			}
			if inner != nil {
				inner(kind, msg)
			}
		}

		start := time.Now()
		answer, err := loop.Run(ctx, system, q.Goal)
		took := time.Since(start).Seconds()

		r := result{q: q, seconds: took, tools: used}
		switch {
		case err != nil:
			r.why = "끝내지 못했다: " + err.Error()
		default:
			r.ok, r.why = score(answer, q)
		}
		mark := "✗"
		if r.ok {
			mark = "✓"
		}
		fmt.Printf("  %s %s  (%.0f초, 도구 %d번)\n", mark, r.why, r.seconds, r.tools)
		rs = append(rs, r)
	}

	return report(rs)
}

// score 는 답에 있어야 할 것이 있고 없어야 할 것이 없는지 본다.
func score(answer string, q Question) (bool, string) {
	low := strings.ToLower(answer)
	var missing []string
	for _, want := range q.Expect {
		if !strings.Contains(low, strings.ToLower(want)) {
			missing = append(missing, want)
		}
	}
	for _, no := range q.Forbid {
		if strings.Contains(low, strings.ToLower(no)) {
			return false, "없어야 할 것이 있다: " + no
		}
	}
	if len(missing) > 0 {
		return false, "빠졌다: " + strings.Join(missing, ", ")
	}
	// 어느 말로 답하든 맞게 재려면 「하나라도」 가 있어야 한다. 영어로 물으면
	// 영어로 답하는데(상용 에이전트도 그렇다), 한국어 글자만 찾으면 멀쩡한
	// 답을 틀렸다고 센다 — 실제로 그렇게 0/2 를 냈다.
	if len(q.Any) > 0 {
		hit := false
		for _, one := range q.Any {
			if strings.Contains(low, strings.ToLower(one)) {
				hit = true
				break
			}
		}
		if !hit {
			return false, "이 가운데 하나는 있어야 한다: " + strings.Join(q.Any, " / ")
		}
	}
	return true, "맞다"
}

func report(rs []result) int {
	ok, totalSec, totalTools := 0, 0.0, 0
	for _, r := range rs {
		if r.ok {
			ok++
		}
		totalSec += r.seconds
		totalTools += r.tools
	}
	n := len(rs)
	fmt.Printf("\n==== 점수 ====\n%d/%d (%d%%) · 평균 %.0f초 · 평균 도구 %.1f번\n",
		ok, n, 100*ok/max1(n), totalSec/float64(max1(n)), float64(totalTools)/float64(max1(n)))
	if ok < n {
		fmt.Println("\n틀린 것")
		for _, r := range rs {
			if !r.ok {
				fmt.Printf("  %-28s %s\n", r.q.Name, r.why)
			}
		}
	}
	if ok < n {
		return 1
	}
	return 0
}

func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
}
