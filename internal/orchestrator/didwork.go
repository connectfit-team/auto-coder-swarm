package orchestrator

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
	"github.com/connectfit-team/auto-coder-swarm/internal/guard"
)

// 시킨 일을 실제로 했는지 본다.
//
// W-47441 이 「고용주웹에서 연결보류 기능을 추가」 를 받아 승인 대기까지
// 갔는데, 넘긴 것이 이것뿐이었다.
//
//	-        picked = picked;
//	+        picked = new Set([...picked]);
//
// 연결보류와 아무 상관이 없다. 계획은 connectcud.ts · connectactions.ts ·
// +page.server.ts · +page.svelte 넷을 짚었는데, 빌드가 깨져 치유가 실패하면서
// 그 넷이 전부 되돌아가고 **엉뚱한 찌꺼기 한 줄만 남았다.** 검토자는 해롭지
// 않으니 통과시켰다. 초록불인데 일은 안 된 것이다.
//
// 판단하지 않고 센다 — 계획이 짚은 자리 가운데 **하나라도** 실제로 바뀌었나.
// 하나도 안 바뀌었으면 한 일이 없는 것이다. 치유가 다른 파일을 고치는 것은
// 정상이므로, 전부가 아니라 하나만 겹치면 된다.

var diffFileRe = regexp.MustCompile(`(?m)^\+\+\+ b/(.+)$`)

// changedFiles 는 diff 가 실제로 건드린 파일이다.
func changedFiles(diff string) []string {
	var out []string
	for _, m := range diffFileRe.FindAllStringSubmatch(diff, -1) {
		if p := strings.TrimSpace(m[1]); p != "" && p != "/dev/null" {
			out = append(out, p)
		}
	}
	return out
}

// CheckDidTheWork 는 계획이 짚은 자리를 하나도 안 고쳤으면 막는다.
// 계획이 비어 있으면 잴 것이 없으므로 막지 않는다.
func CheckDidTheWork(plan agent.Plan, diff string) []guard.Violation {
	var planned []string
	for _, c := range plan.Changes {
		if p := strings.TrimSpace(c.FilePath); p != "" {
			planned = append(planned, p)
		}
	}
	if len(planned) == 0 || strings.TrimSpace(diff) == "" {
		return nil
	}

	touched := map[string]bool{}
	for _, f := range changedFiles(diff) {
		touched[f] = true
		touched[filepath.Base(f)] = true
	}
	for _, p := range planned {
		if touched[p] || touched[filepath.Base(p)] {
			return nil
		}
	}

	return []guard.Violation{{
		Why: "계획이 짚은 자리를 하나도 고치지 않았다 — 시킨 일을 한 것이 아니다",
		Evidence: append(
			[]string{"계획: " + strings.Join(clipList(planned), " · ")},
			"고친 것: "+strings.Join(clipList(changedFiles(diff)), " · ")),
	}}
}
