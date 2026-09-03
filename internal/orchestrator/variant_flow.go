package orchestrator

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/connectfit-team/auto-coder-swarm/internal/gitmgr"
	"github.com/connectfit-team/auto-coder-swarm/internal/insightclient"
	"github.com/connectfit-team/auto-coder-swarm/internal/korean"
)

// 값 하나를 더하는 작업을 저장소마다 돈다.
//
// 순서가 강제된다. protogen 의 proto 가 배포돼야 소비자가 컴파일되므로,
// 막는 저장소의 PR 이 열리면 거기서 멈추고 사람에게 넘긴다. 앞이 머지되기
// 전에 뒤를 밀면 소비자 PR 이 빌드에서 깨진다.

// VariantResult 는 저장소 하나의 결과다.
type VariantResult struct {
	Repo        string
	PRURL       string
	Files       []string
	Inserted    int
	Skipped     int
	NeedsManual []string
	Err         string
}

const variantPlanTimeout = 3 * time.Minute

// runVariantAddition 은 계획을 받아 저장소마다 적용하고 PR 을 연다.
func (t *taskContext) runVariantAddition(req insightclient.VariantPlanRequest) ([]VariantResult, error) {
	sub, cancel := context.WithTimeout(t.ctx, variantPlanTimeout)
	plans, err := t.orchestrator.insightClient.VariantPlan(sub, req)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("계획을 못 받았다: %w", err)
	}
	if len(plans) == 0 {
		return nil, fmt.Errorf("고칠 저장소를 하나도 못 찾았다")
	}

	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "VARIANT_PLAN",
		fmt.Sprintf("저장소 %d개에 %s 더한다", len(plans), korean.With(req.Value, "을", "를")), "", planSummaryText(plans))

	return t.applyPlans(plans, req), nil
}

func (t *taskContext) applyOneRepo(p insightclient.VariantRepoPlan, req insightclient.VariantPlanRequest, blockers []string) VariantResult {
	r := VariantResult{Repo: p.Repo, NeedsManual: p.NeedsManual}

	if url := existingVariantPR(p.Repo, req.Value); url != "" {
		r.PRURL = url
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "VARIANT_PR_EXISTS",
			fmt.Sprintf("%s 에는 같은 PR 이 이미 열려 있다 — 그것을 쓴다", p.Repo), "", url)
		return r
	}

	branch := fmt.Sprintf("feat/add-%s-%s", req.Value, t.taskID)
	repoPath := filepath.Join(t.wsPath, p.Repo)

	// 사본 경로가 저장소 이름과 다를 수 있다 — 서브모듈이 그렇다.
	source := p.SourcePath
	if source == "" {
		source = p.Repo
	}
	if err := t.orchestrator.wsMgr.CreateWorktree(source, repoPath, branch); err != nil {
		r.Err = fmt.Sprintf("작업공간을 못 만들었다: %v", err)
		return r
	}

	out, err := applyVariantPlan(repoPath, p)
	if err != nil {
		r.Err = fmt.Sprintf("적용 실패: %v", err)
		return r
	}
	r.Files, r.Inserted, r.Skipped = out.Files, out.Inserted, out.Skipped

	if r.Inserted == 0 {
		// 이미 다 들어 있다. PR 을 열 이유가 없다.
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "VARIANT_NOOP",
			fmt.Sprintf("%s 에는 이미 다 들어 있다 (건너뜀 %d)", p.Repo, r.Skipped), "", "")
		return r
	}

	if msg := verifyRepo(repoPath, p.Repo); msg != "" {
		r.Err = fmt.Sprintf("검증 실패: %s", msg)
		return r
	}

	msg := variantCommitMessage(req, p)
	url, err := t.orchestrator.gitMgr.PushApprovedChangesOpt(repoPath, p.Repo, branch, msg,
		gitmgr.PushOptions{
			Title:    fmt.Sprintf("%s %s 더한다", p.Repo, korean.With(req.Label, "을", "를")),
			BodyLead: blockerNote(blockers),
			Draft:    len(blockers) > 0,
		})
	if err != nil {
		r.Err = fmt.Sprintf("PR 을 못 열었다: %v", err)
		r.PRURL = url // 브랜치는 올라갔을 수 있다. 주소를 버리지 않는다.
		return r
	}
	r.PRURL = url
	return r
}

// verifyRepo 는 볼 수 있는 것만 본다. 못 보는 것은 통과시킨다 —
// 검증 못 한 것을 실패로 적으면 멀쩡한 PR 이 막힌다.
func verifyRepo(path, repo string) string {
	if _, err := exec.LookPath("gofmt"); err == nil {
		cmd := exec.Command("bash", "-c",
			`find . -name '*.go' -not -path './vendor/*' -print0 | xargs -0 -r gofmt -e -l`)
		cmd.Dir = path
		if b, err := cmd.CombinedOutput(); err == nil && len(strings.TrimSpace(string(b))) > 0 {
			return "gofmt 가 읽지 못하는 파일: " + firstLineOf(string(b))
		}
	}
	// proto 는 중괄호가 맞는지만 본다. protoc 은 여기 없다.
	cmd := exec.Command("bash", "-c",
		`for f in $(find . -name '*.proto'); do `+
			`a=$(grep -o '{' "$f"|wc -l); b=$(grep -o '}' "$f"|wc -l); `+
			`[ "$a" = "$b" ] || echo "$f"; done`)
	cmd.Dir = path
	if b, err := cmd.CombinedOutput(); err == nil && len(strings.TrimSpace(string(b))) > 0 {
		return "중괄호가 안 맞는 proto: " + firstLineOf(string(b))
	}
	return ""
}

func variantCommitMessage(req insightclient.VariantPlanRequest, p insightclient.VariantRepoPlan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s 더한다\n\n", korean.With(req.Label, "을", "를"))
	fmt.Fprintf(&b, "%s 가 있는 자리마다 %s 몫을 나란히 넣는다.\n", req.Seed, req.Value)
	if p.Note != "" {
		fmt.Fprintf(&b, "\n%s\n", p.Note)
	}
	if len(p.NeedsManual) > 0 {
		fmt.Fprintf(&b, "\n저장소에 없는 이름이 있다 — 사람이 채워야 한다:\n")
		for _, n := range p.NeedsManual {
			fmt.Fprintf(&b, "  %s\n", n)
		}
	}
	return b.String()
}

func planSummaryText(plans []insightclient.VariantRepoPlan) string {
	var b strings.Builder
	for _, p := range plans {
		fmt.Fprintf(&b, "%d) %s — 자리 %d곳", p.Order, p.Repo, len(p.Changes))
		if p.Blocks {
			b.WriteString(" (이게 먼저 배포돼야 함)")
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func firstLineOf(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
