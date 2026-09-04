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

// tryVariantAddition 은 값 추가 요청이면 그 길로 가고, 아니면 넘긴다.
// 두 번째 값이 false 면 결함 흐름이 이어받는다.
func (t *taskContext) tryVariantAddition() (RunResult, bool, error) {
	sub, cancel := context.WithTimeout(t.ctx, variantAskTimeout)
	ask, err := t.orchestrator.insightClient.VariantAsk(sub, t.req.UserRequest, []string{"clockio"})
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

	res := RunResult{RepoName: firstRepo(ask.Plans)}
	var urls []string
	for _, r := range results {
		if r.PRURL != "" {
			urls = append(urls, r.Repo+": "+r.PRURL)
		}
	}
	if len(urls) == 0 {
		return res, true, fmt.Errorf("PR 을 하나도 못 열었다: %s", firstError(results))
	}
	res.PRURL = strings.Join(urls, "\n")

	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "VARIANT_DONE",
		fmt.Sprintf("PR %d개를 열었다", len(urls)), "", resultSummary(results))
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

	for _, p := range plans {
		if p.Publish == "protogen-make" {
			// proto 는 PR 이 아니라 protogen 의 make 목표로 배포한다.
			// 그것이 컴파일·커밋·push 를 한다. 여기서 PR 을 열면 안 된다.
			out = append(out, t.recordProtoPublish(p))
			blockers = append(blockers,
				fmt.Sprintf("%s — protogen 에서 `make %s` (사람이 돌린다)", p.Repo, p.MakeTarget))
			continue
		}

		r := t.applyOneRepo(p, req, blockers)
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

// blockerNote 는 먼저 머지돼야 하는 PR 을 본문 맨 앞에 적는다.
func blockerNote(blockers []string) string {
	if len(blockers) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("> 먼저 머지·배포돼야 이 PR 이 빌드된다:\n>\n")
	for _, x := range blockers {
		fmt.Fprintf(&b, "> - %s\n", x)
	}
	b.WriteString(">\n> 그때까지 초안으로 둔다.\n\n")
	return b.String()
}

func askSummary(ask insightclient.VariantAskResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "씨앗 %s · 더할 값 %s (%s)\n", ask.Seed, ask.Value, ask.Label)
	for _, p := range ask.Plans {
		fmt.Fprintf(&b, "%d) %s — 자리 %d곳", p.Order, p.Repo, len(p.Changes))
		if p.AlreadyThere > 0 {
			fmt.Fprintf(&b, " (이미 있음 %d곳)", p.AlreadyThere)
		}
		if p.Blocks {
			b.WriteString(" (이게 먼저 배포돼야 함)")
		}
		if len(p.NeedsManual) > 0 {
			fmt.Fprintf(&b, " · 없는 이름: %s", strings.Join(p.NeedsManual, ", "))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func resultSummary(rs []VariantResult) string {
	var b strings.Builder
	for _, r := range rs {
		fmt.Fprintf(&b, "%s — 넣음 %d · 건너뜀 %d", r.Repo, r.Inserted, r.Skipped)
		if len(r.Unverified) > 0 {
			fmt.Fprintf(&b, " · 문법 확인 못 함: %s", strings.Join(r.Unverified, " "))
		}
		if r.PRURL != "" {
			fmt.Fprintf(&b, " · %s", r.PRURL)
		}
		if r.Err != "" {
			fmt.Fprintf(&b, " · 실패: %s", r.Err)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func firstRepo(plans []insightclient.VariantRepoPlan) string {
	if len(plans) > 0 {
		return plans[0].Repo
	}
	return ""
}

func firstError(rs []VariantResult) string {
	for _, r := range rs {
		if r.Err != "" {
			return r.Err
		}
	}
	return "넣을 것이 하나도 없었다"
}

// recordProtoPublish 는 proto 를 어떻게 배포해야 하는지 남긴다.
// 실제 배포는 사람이 protogen 에서 make 로 한다 — 그것이 main 에 바로 민다.
func (t *taskContext) recordProtoPublish(p insightclient.VariantRepoPlan) VariantResult {
	var b strings.Builder
	fmt.Fprintf(&b, "protogen 에서 `make %s` 을 돌린다. 그것이 컴파일·커밋·push 를 한다.\n", p.MakeTarget)
	b.WriteString("넣을 것:\n")
	for _, c := range p.Changes {
		fmt.Fprintf(&b, "  %s:%d 다음에\n", c.File, c.InsertAfter)
		for _, l := range c.Block {
			fmt.Fprintf(&b, "    %s\n", l)
		}
	}
	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "VARIANT_PROTO_PUBLISH",
		fmt.Sprintf("%s 는 make %s 로 배포한다 — PR 이 아니다", p.Repo, p.MakeTarget),
		"", b.String())
	return VariantResult{Repo: p.Repo, NeedsManual: p.NeedsManual, Inserted: len(p.Changes)}
}

// coverageTable 은 저장소마다 담았는지, 뺐으면 왜인지 적는다.
// 뺀 것을 보여 주지 않으면 놓친 것인지 사람이 알 수 없다.
func coverageTable(vs []insightclient.RepoVerdict) string {
	var b strings.Builder
	for _, v := range vs {
		if v.Planned > 0 {
			fmt.Fprintf(&b, "담음  %-24s %d곳 (낱말 %d회)\n", v.Repo, v.Planned, v.Hits)
			continue
		}
		fmt.Fprintf(&b, "뺌    %-24s 낱말 %d회 — %s\n", v.Repo, v.Hits, v.Reason)
		if v.Evidence != "" {
			fmt.Fprintf(&b, "        %s\n", v.Evidence)
		}
	}
	return b.String()
}
