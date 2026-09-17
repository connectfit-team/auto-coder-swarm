package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// candidateFiles 는 이 일과 맞닿은 파일들이다. 분석이 짚은 것을 쓴다.
func (t *taskContext) candidateFiles() []string {
	var out []string
	for _, p := range pathsInText(t.analysis) {
		out = append(out, p)
		if len(out) >= 6 {
			break
		}
	}
	return out
}

// chainForMissingState 는 담을 자리가 없을 때 그 계약의 임자에게 일을 만든다.
//
// 임자를 모르면 만들지 않는다 — 없는 저장소에 일을 만들면 시작하자마자 죽고
// 사람은 까닭 없는 실패를 하나 더 볼 뿐이다.
func (t *taskContext) chainForMissingState(missing []string) []StatelessRequest {
	if t.req.Depth <= 0 || len(missing) == 0 {
		return nil
	}
	q := fmt.Sprintf("%s 에 %s 이(가) 필요하다. %s",
		t.targetRepo, strings.Join(missing, ", "), strings.TrimSpace(t.req.UserRequest))

	sub, cancel := context.WithTimeout(t.ctx, 30*time.Second)
	defer cancel()
	routed, err := t.orchestrator.insightClient.RouteRepos(sub, q)
	if err != nil {
		return nil
	}
	for _, r := range routed {
		if r.RepoName == t.targetRepo || hasRepo(t.req.ParentRepos, r.RepoName) {
			continue
		}
		if !t.orchestrator.wsMgr.HasRepo(r.RepoName) {
			continue
		}
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CHAIN_TRIGGERED",
			fmt.Sprintf("담을 자리를 만들 저장소에 넘긴다: %s", r.RepoName), "", strings.Join(missing, " · "))
		return []StatelessRequest{{
			UserRequest: fmt.Sprintf(
				"%s 에서 「%s」 를 만들려는데 담을 자리가 없어 막혔다:\n  %s\n"+
					"이 저장소에 먼저 있어야 한다. 상태를 담을 필드와 그것을 바꾸는 길을 더해라.",
				t.targetRepo, strings.TrimSpace(t.req.UserRequest), strings.Join(missing, "\n  ")),
			TargetRepo:   r.RepoName,
			Depth:        t.req.Depth - 1,
			ParentRepos:  append(t.req.ParentRepos, t.targetRepo),
			ParentTaskID: rootTaskID(t),
		}}
	}
	return nil
}
