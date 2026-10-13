package main

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/connectfit-team/auto-coder-swarm/internal/orchestrator"
)

// **모형이 관문에 실제로 걸리는지 LLM 없이 먼저 본다.**
//
// 안 걸리는 모형으로 채점하면 LLM 시간을 다 쓰고서야 「시험이 잘못됐다」 를
// 본다. 실제로 그랬다 — mixed-type 은 두 필드가 같은 수정 안에 있어야 하고,
// suffix-name 은 이름 버릇을 스무 개 넘게 봐야 센다(#248).
func TestEveryFixtureActuallyTripsAGate(t *testing.T) {
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo, err := fixture(c.head, c.bad)
			if err != nil {
				t.Fatalf("모형을 못 만들었다: %v", err)
			}
			defer os.RemoveAll(repo)

			diff := worktreeDiff(context.Background(), repo)
			if strings.TrimSpace(diff) == "" {
				t.Fatal("고친 것이 없다 — bad 가 head 와 같다")
			}
			// 채점기의 check 와 **같은 관문들**을 본다. 하나라도 빠지면 모형이
			// 안 걸리는 것을 못 잡는다.
			bad := append(orchestrator.CheckProtoChange(repo, diff),
				orchestrator.CheckStateNeedsRPC(repo, diff)...)
			if len(bad) == 0 {
				t.Fatalf("관문에 안 걸린다 — 이 모형으로는 아무것도 못 잰다.\ndiff:\n%s", clip(diff, 400))
			}
			t.Logf("걸린다: %s", clip(orchestrator.AlignmentNote(bad), 160))
		})
	}
}

// 고친 뒤(= head 로 되돌린 뒤)에는 관문이 조용해야 한다. 안 그러면 모형이
// 고칠 수 없는 것을 내는 셈이다.
func TestFixtureIsFixable(t *testing.T) {
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo, err := fixture(c.head, nil)
			if err != nil {
				t.Fatalf("모형을 못 만들었다: %v", err)
			}
			defer os.RemoveAll(repo)
			d := worktreeDiff(context.Background(), repo)
			bad := append(orchestrator.CheckProtoChange(repo, d),
				orchestrator.CheckStateNeedsRPC(repo, d)...)
			if len(bad) > 0 {
				t.Errorf("고치기 전부터 head 가 관문에 걸린다: %s",
					clip(orchestrator.AlignmentNote(bad), 200))
			}
		})
	}
}
