package orchestrator

import (
	"testing"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
)

func planOf(files ...string) agent.Plan {
	var p agent.Plan
	for _, f := range files {
		p.Changes = append(p.Changes, agent.FileChange{FilePath: f})
	}
	return p
}

func TestCheckDidTheWork(t *testing.T) {
	plan := planOf(
		"src/lib/server/data/connectcud.ts",
		"src/lib/types/connectactions.ts",
		"src/routes/(auth)/store/connect/+page.svelte",
	)

	// W-47441 이 넘긴 것 — 계획에 있던 파일이긴 하다. 막지 않는다.
	same := "--- a/src/routes/(auth)/store/connect/+page.svelte\n+++ b/src/routes/(auth)/store/connect/+page.svelte\n@@\n+    picked = new Set([...picked]);\n"
	if bad := CheckDidTheWork(plan, same); len(bad) != 0 {
		t.Errorf("계획에 있던 파일인데 막았다: %v", bad)
	}

	// 계획과 아무 상관 없는 파일만 고쳤다.
	other := "--- a/src/lib/util/retry.ts\n+++ b/src/lib/util/retry.ts\n@@\n+    const x = 1;\n"
	if bad := CheckDidTheWork(plan, other); len(bad) == 0 {
		t.Error("계획이 짚은 자리를 하나도 안 고쳤는데 통과시켰다")
	}

	// 계획이 없으면 잴 것이 없다.
	if bad := CheckDidTheWork(agent.Plan{}, other); len(bad) != 0 {
		t.Errorf("계획이 없는데 막았다: %v", bad)
	}

	// 빈 diff 는 다른 곳에서 막는다. 여기서는 잴 것이 없다.
	if bad := CheckDidTheWork(plan, ""); len(bad) != 0 {
		t.Errorf("빈 diff 를 여기서 막았다: %v", bad)
	}
}

func TestChangedFiles(t *testing.T) {
	d := "--- a/x.ts\n+++ b/x.ts\n@@\n+a\n--- a/y.ts\n+++ b/y.ts\n@@\n+b\n"
	got := changedFiles(d)
	if len(got) != 2 || got[0] != "x.ts" || got[1] != "y.ts" {
		t.Errorf("파일 목록이 틀렸다: %v", got)
	}
}
