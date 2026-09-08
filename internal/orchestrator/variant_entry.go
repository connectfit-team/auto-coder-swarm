package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/connectfit-team/auto-coder-swarm/internal/insightclient"
	"github.com/connectfit-team/auto-coder-swarm/internal/korean"
)

// 값 추가 요청은 결함 흐름과 길이 다르다.
//
// 결함은 "어디가 잘못됐나" 를 찾아 한 저장소를 고친다. 값 추가는 "이미 있는
// 값이 어디에 쓰이나" 를 따라가 여러 저장소를 나란히 고친다. 계획을 세우는
// 방법도, PR 을 여는 순서도 다르다.

const variantAskTimeout = 5 * time.Minute

// protoPublish 는 그 저장소가 PR 이 아니라 protogen 의 make 목표로 배포된다는 표시다.
const protoPublish = "protogen-make"

// clockio 는 같은 회사의 다른 서비스다. 낱말이 겹쳐도 gig 앱 요청에
// 끌어들이면 안 된다.
var reposOutOfScope = []string{"clockio"}

// tryVariantAddition 은 값 추가 요청이면 그 길로 가고, 아니면 넘긴다.
// 두 번째 값이 false 면 결함 흐름이 이어받는다.
func (t *taskContext) tryVariantAddition() (RunResult, bool, error) {
	sub, cancel := context.WithTimeout(t.ctx, variantAskTimeout)
	ask, err := t.orchestrator.insightClient.VariantAsk(sub, t.req.UserRequest, reposOutOfScope)
	cancel()
	if errors.Is(err, insightclient.ErrNotAuthorized) {
		// 열쇠가 틀린 것을 "값 추가가 아니다" 로 넘기면, 설정 문제가 판단
		// 문제로 위장돼 엉뚱한 흐름이 조용히 돈다. 여기서 멈춘다.
		return RunResult{}, true, fmt.Errorf("CIE 에 물어보지 못했다 — CIE_API_KEY 를 확인해라: %w", err)
	}
	if err != nil {
		// 물어보지 못한 것과 값 추가가 아닌 것은 다르다. 결함 흐름으로 넘긴다.
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "VARIANT_ASK_FAILED",
			"값 추가인지 못 물어봤다 — 결함 흐름으로 간다", "", err.Error())
		return RunResult{}, false, nil
	}
	if !ask.IsVariantAddition {
		return RunResult{}, false, nil
	}
	if ask.Error != "" || len(ask.Plans) == 0 {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "VARIANT_NO_PLAN",
			"값 추가로 읽었지만 고칠 자리를 못 찾았다", "", ask.Error)
		return RunResult{}, false, nil
	}

	t.namedRepos = ask.NamedRepos
	if len(ask.NamedRepos) > 0 {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "VARIANT_SCOPE",
			fmt.Sprintf("요청이 저장소를 지목했다 — %s 만 고친다",
				strings.Join(ask.NamedRepos, ", ")),
			"", "proto 는 지목되지 않았어도 배포돼야 나머지가 컴파일된다.")
	}
	if len(ask.Coverage) > 0 {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "VARIANT_COVERAGE",
			fmt.Sprintf("%s 가 나오는 저장소 %d개를 봤고 %d개를 고친다",
				ask.Seed, len(ask.Coverage), len(ask.Plans)),
			"", coverageTable(ask.Coverage))
	}

	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "VARIANT_DETECTED",
		fmt.Sprintf("%s 더한다 — %s 있는 자리를 따라간다",
			korean.With(ask.Value, "을", "를"), korean.With(ask.Seed, "이", "가")),
		"", askSummary(ask))

	wsPath, err := t.orchestrator.wsMgr.CreateWorkspace()
	if err != nil {
		return RunResult{}, true, fmt.Errorf("작업공간을 못 만들었다: %w", err)
	}
	t.wsPath = wsPath
	defer t.orchestrator.wsMgr.Cleanup(wsPath)

	results := t.applyPlans(ask.Plans, insightclient.VariantPlanRequest{
		Seed: ask.Seed, Value: ask.Value, Label: ask.Label,
	})

	// 결과 글이 비면 아무것도 못 한 것이다. proto 만 고칠 것이 있는 요청은
	// PR 이 없어도 성공이다 — PR 수로만 재면 올바른 결과가 실패로 보고된다.
	res := RunResult{RepoName: firstRepo(ask.Plans), Result: outcomeText(results)}
	if t.steerStopped {
		// 멈춘 것을 완료로 보이게 하면 안 된다. 무엇을 안 했는지 적는다.
		res.Result = strings.TrimSpace(res.Result + "\n(도중에 멈추라고 해서 남은 저장소는 하지 않았다)")
	}
	if strings.TrimSpace(res.Result) == "(도중에 멈추라고 해서 남은 저장소는 하지 않았다)" {
		return res, true, fmt.Errorf("사람이 멈춰서 아무것도 올리지 않았다")
	}
	if res.Result == "" {
		return res, true, fmt.Errorf("PR 을 하나도 못 열었다: %s", whyNoPR(results))
	}

	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "VARIANT_DONE",
		doneHeadline(results), "", resultSummary(results))
	return res, true, nil
}

// applyPlans 는 계획에 있는 저장소를 모두 연다.
//
// 먼저 배포돼야 하는 저장소(proto)가 있어도 거기서 멈추지 않는다. 멈추면
// 사람은 나머지 저장소에 무엇이 필요한지 못 보고, 그것이 요청의 전부다.
// 대신 뒤따르는 PR 은 초안으로 열고 무엇이 먼저 머지돼야 하는지 본문에 적는다.
//
// 한 저장소가 실패해도 나머지는 계속한다. 서로 의존하지 않는다.
func (t *taskContext) applyPlans(plans []insightclient.VariantRepoPlan, req insightclient.VariantPlanRequest) []VariantResult {
	var out []VariantResult
	var blockers []string
	pending := PendingSymbols(plans)

	for _, p := range plans {
		// 걸음마다 사람의 말을 듣는다. 이미 나간 것은 되돌리지 않는다 —
		// 열린 PR 은 열린 채로 남고, 다음 저장소부터 반영한다.
		if act, ok := t.takeSteers(remainingRepos(plans, out)); ok {
			if act.Stop {
				t.steerStopped = true
				t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "STEER_STOP",
					"사람이 멈추라고 했다 — 남은 저장소는 건드리지 않는다", "",
					strings.Join(t.steerNotes, "\n"))
				break
			}
			if !keepRepo(p.Repo, act) {
				t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "STEER_SKIP",
					fmt.Sprintf("사람이 %s 는 하지 말라고 했다", p.Repo), "",
					strings.Join(t.steerNotes, "\n"))
				continue
			}
		}
		if p.Publish == protoPublish {
			// proto 는 PR 이 아니라 protogen 의 make 목표로 배포한다.
			// 그것이 컴파일·커밋·push 를 한다. 여기서 PR 을 열면 안 된다.
			out = append(out, t.recordProtoPublish(p))
			blockers = append(blockers,
				fmt.Sprintf("%s — protogen 에서 `make %s` (사람이 돌린다)", p.Repo, p.MakeTarget))
			continue
		}

		r := t.applyOneRepo(p, req, blockers, pending)
		out = append(out, r)

		if r.Err != "" {
			t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "VARIANT_REPO_FAILED",
				fmt.Sprintf("%s 실패 — 나머지는 계속한다", korean.With(p.Repo, "이", "가")), "", r.Err)
			continue
		}
		if p.Blocks && r.PRURL != "" {
			blockers = append(blockers, p.Repo+" "+r.PRURL)
			t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "VARIANT_BLOCKER",
				fmt.Sprintf("%s 머지·배포돼야 나머지가 컴파일된다 — 뒤 PR 은 초안으로 연다",
					korean.With(p.Repo, "이", "가")), "", p.Note)
		}
	}
	return out
}

// recordProtoPublish 는 proto 를 어떻게 배포해야 하는지 남긴다.
// 실제 배포는 사람이 protogen 에서 make 로 한다 — 그것이 main 에 바로 민다.
func (t *taskContext) recordProtoPublish(p insightclient.VariantRepoPlan) VariantResult {
	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "VARIANT_PROTO_PUBLISH",
		fmt.Sprintf("%s 는 make %s 로 배포한다 — PR 이 아니다", p.Repo, p.MakeTarget),
		"", protoPublishSteps(p))
	return VariantResult{
		Repo:        p.Repo,
		MakeTarget:  p.MakeTarget,
		NeedsManual: p.NeedsManual,
		Inserted:    len(p.Changes),
	}
}

func firstRepo(plans []insightclient.VariantRepoPlan) string {
	if len(plans) > 0 {
		return plans[0].Repo
	}
	return ""
}

// takeSteers 는 큐에 들어온 사람의 말을 집어 온다.
// 집어 온 것이 있으면 두 번째 값이 true 다.
func (t *taskContext) takeSteers(repos []string) (SteerAction, bool) {
	pending, err := t.orchestrator.store.PendingSteers(t.taskID)
	if err != nil || len(pending) == 0 {
		return SteerAction{}, false
	}
	var ids []uint
	var merged SteerAction
	for _, s := range pending {
		act := ParseSteer(s.Message, repos)
		merged.Stop = merged.Stop || act.Stop
		merged.Only = append(merged.Only, act.Only...)
		merged.Exclude = append(merged.Exclude, act.Exclude...)
		t.steerNotes = appendOnceStr(t.steerNotes, act.Note)
		ids = append(ids, s.ID)
	}
	_ = t.orchestrator.store.MarkSteersApplied(ids)
	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "STEER_TAKEN",
		fmt.Sprintf("도중에 들어온 말 %d개를 집었다", len(pending)), "",
		steerSummary(merged, t.steerNotes))
	return merged, true
}

// remainingRepos 는 아직 손대지 않은 저장소 이름을 준다.
// 사람의 말에서 저장소 이름을 찾을 때 이것과 견준다.
func remainingRepos(plans []insightclient.VariantRepoPlan, done []VariantResult) []string {
	var out []string
	for _, p := range plans {
		seen := false
		for _, d := range done {
			if d.Repo == p.Repo {
				seen = true
			}
		}
		if !seen {
			out = appendOnceStr(out, p.Repo)
		}
	}
	return out
}

// keepRepo 는 사람의 말대로 이 저장소를 계속할지 본다.
func keepRepo(repo string, act SteerAction) bool {
	return len(KeepSteered([]string{repo}, act)) == 1
}
