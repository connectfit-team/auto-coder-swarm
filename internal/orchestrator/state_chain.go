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
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CHAIN_SKIPPED",
			"저장소 고르기가 답하지 않았다", err.Error(), strings.Join(missing, " · "))
		return nil
	}
	// 걸러낸 것은 까닭과 함께 남긴다. 조용히 비우면 「못 골랐다」 만 남아
	// 무엇을 보고 그랬는지 알 수 없다.
	var dropped []string
	for _, r := range routed {
		if r.RepoName == t.targetRepo || hasRepo(t.req.ParentRepos, r.RepoName) {
			dropped = append(dropped, r.RepoName+"(이미 거쳐 온 저장소)")
			continue
		}
		if !t.orchestrator.wsMgr.HasRepo(r.RepoName) {
			dropped = append(dropped, r.RepoName+"(사본이 없다)")
			continue
		}
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CHAIN_TRIGGERED",
			fmt.Sprintf("담을 자리를 만들 저장소에 넘긴다: %s", r.RepoName), "저장소 고르기가 골랐다", strings.Join(missing, " · "))
		return []StatelessRequest{t.stateChainRequest(r.RepoName, missing)}
	}
	// **왜 못 만들었는지 적는다.** 조용히 비우면 사람은 연쇄가 도는 줄 안다.
	why := "저장소 고르기가 아무것도 내놓지 않았다"
	if len(dropped) > 0 {
		why = "걸러낸 것: " + strings.Join(dropped, ", ")
	}
	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CHAIN_SKIPPED",
		"담을 자리를 만들 저장소를 못 골랐다", why, strings.Join(missing, " · "))
	return nil
}

// stateChainRequest 는 그 저장소에 남길 일이다.
//
// **넘기는 쪽이 아는 것을 다 싣는다.** 부모는 이미 어느 파일·어느 계약·어떤
// 발행 목표인지 알아냈는데, 그것을 빼고 「gig_ceo_web 에서 …」 로 시작하는
// 산문만 넘겼다. 그래서 자식이 대상을 다시 고르고 엉뚱한 저장소를 뒤지다
// 넷 다 0줄로 죽었다.
//
// 첫 줄이 **할 일**이어야 한다 — 어디서 막혔는지는 배경으로 뒤에 둔다.
func (t *taskContext) stateChainRequest(owner string, missing []string) StatelessRequest {
	var where string
	if t.protoPath != "" {
		where = fmt.Sprintf("고칠 자리: %s\n펴내기: %s\n", t.protoPath, t.protoTarget)
	}
	return StatelessRequest{
		UserRequest: fmt.Sprintf(
			"%s 저장소에 이것을 더해라: %s\n%s"+
				"상태를 담을 필드와 그것을 바꾸는 길(RPC)을 함께 더한다.\n"+
				"**계약은 원본(.proto)만 고친다.** 펴낸 결과물(*.pb.go·생성된 .ts)은 손대지 마라 — "+
				"다음 발행 때 덮어써진다.\n\n"+
				"[배경] %s 에서 「%s」 를 만들려는데 담을 자리가 없어 막혔다.",
			owner, strings.Join(missing, ", "), where,
			t.targetRepo, strings.TrimSpace(t.req.UserRequest)),
		TargetRepo:   owner,
		AddsState:    true,
		Depth:        t.req.Depth - 1,
		ParentRepos:  append(t.req.ParentRepos, t.targetRepo),
		ParentTaskID: rootTaskID(t),
	}
}
