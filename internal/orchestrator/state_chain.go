package orchestrator

import (
	"context"
	"fmt"
	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
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
	if len(missing) == 0 {
		return nil
	}
	if t.req.Depth <= 0 {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CHAIN_SKIPPED",
			"연쇄 깊이가 남지 않아 다른 저장소에 일을 만들지 않는다", "", "")
		return nil
	}

	// **타입에서 임자를 찾는다.** 산문으로 저장소를 고르게 하면 빗나간다.
	if plan, ok := t.ctx.Value("current_plan").(agent.Plan); ok {
		var files []string
		for _, c := range plan.Changes {
			files = append(files, c.FilePath)
		}
		if owner, why := t.ownerRepoForState(files, missing); owner != "" {
			t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CHAIN_TRIGGERED",
				fmt.Sprintf("담을 자리를 만들 저장소에 넘긴다: %s", owner), why, strings.Join(missing, " · "))
			return []StatelessRequest{t.stateChainRequest(owner, missing)}
		} else if why != "" {
			t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CHAIN_SKIPPED",
				"타입에서 임자를 못 찾았다 — 저장소 고르기로 물어본다", why, "")
		}
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
			fmt.Sprintf("담을 자리를 만들 저장소에 넘긴다: %s", r.RepoName), "저장소 고르기가 골랐다", strings.Join(missing, " · "))
		return []StatelessRequest{t.stateChainRequest(r.RepoName, missing)}
	}
	// **왜 못 만들었는지 적는다.** 조용히 비우면 사람은 연쇄가 도는 줄 안다.
	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CHAIN_SKIPPED",
		"담을 자리를 만들 저장소를 못 골랐다", "", strings.Join(missing, " · "))
	return nil
}

// stateChainRequest 는 그 저장소에 남길 일이다.
func (t *taskContext) stateChainRequest(owner string, missing []string) StatelessRequest {
	return StatelessRequest{
		UserRequest: fmt.Sprintf(
			"%s 에서 「%s」 를 만들려는데 담을 자리가 없어 막혔다:\n  %s\n"+
				"이 저장소에 먼저 있어야 한다. 상태를 담을 필드와 그것을 바꾸는 길을 더해라.\n"+
				"**계약은 원본(.proto)만 고친다.** 펴낸 결과물(*.pb.go·생성된 .ts)은 손대지 마라 — "+
				"다음 발행 때 덮어써진다. 고친 뒤 발행은 protogen 의 make 목표로 한다.",
			t.targetRepo, strings.TrimSpace(t.req.UserRequest), strings.Join(missing, "\n  ")),
		TargetRepo:   owner,
		Depth:        t.req.Depth - 1,
		ParentRepos:  append(t.req.ParentRepos, t.targetRepo),
		ParentTaskID: rootTaskID(t),
	}
}
