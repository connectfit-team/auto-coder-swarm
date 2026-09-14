package orchestrator

import (
	"sort"
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
)

// ReviewFacts 는 검토자에게 줄 **기계가 파일을 읽어 확인한 사실**이다.
//
// 검토자는 이름이 실제로 있는지, 시킨 일을 했는지 알 방법이 없어서 인상으로
// 답했다. 라벨 문항 셋에서 0/3 — 나쁜 것은 통과시키고 좋은 것은 막았다.
// 여기에 담는 것은 전부 센 것이라 틀릴 수가 없다.
func ReviewFacts(repoPath string, planned []string, diff string) string {
	changed := changedFiles(diff)
	if len(changed) == 0 {
		return ""
	}

	var b strings.Builder
	if len(planned) > 0 {
		sort.Strings(planned)
		b.WriteString("계획이 짚은 자리: " + strings.Join(clipNames(planned, 10), ", ") + "\n")
		b.WriteString("실제로 고친 자리: " + strings.Join(clipNames(changed, 10), ", ") + "\n")
		if len(CheckDidTheWork(planFromFiles(planned), diff)) > 0 {
			b.WriteString("→ 계획이 짚은 자리를 **하나도** 고치지 않았다.\n")
		}
		b.WriteString("\n")
	}

	// 고친 파일마다 쓸 수 있는 이름. 여기 없는 것을 부르면 없는 이름이다.
	n := 0
	for _, f := range changed {
		if n >= 3 {
			break
		}
		if sheet := AvailableNames(repoPath, f); sheet != "" {
			b.WriteString(f + " 에서 쓸 수 있는 이름\n" + sheet)
			n++
		}
	}
	return b.String()
}

// planFromFiles 는 파일 목록만으로 계획 모양을 만든다 — CheckDidTheWork 가
// 파일만 보기 때문이다.
func planFromFiles(files []string) agent.Plan {
	var p agent.Plan
	for _, f := range files {
		p.Changes = append(p.Changes, agent.FileChange{FilePath: f})
	}
	return p
}
