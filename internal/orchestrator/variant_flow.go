package orchestrator

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/connectfit-team/auto-coder-swarm/internal/gitmgr"
	"github.com/connectfit-team/auto-coder-swarm/internal/insightclient"
	"github.com/connectfit-team/auto-coder-swarm/internal/korean"
)

// 값 하나를 더하는 작업을 저장소마다 돈다.
//
// proto 가 배포돼야 소비자가 컴파일되지만 거기서 멈추지 않는다 — 멈추면
// 사람은 나머지 저장소에 무엇이 필요한지 못 보고, 그것이 요청의 전부다.
// 뒤따르는 PR 은 초안으로 열고 무엇이 먼저 배포돼야 하는지 본문에 적는다.

// VariantResult 는 저장소 하나의 결과다.
type VariantResult struct {
	Repo  string
	PRURL string
	// proto 는 PR 이 아니라 protogen 의 make 목표로 배포한다 — 그 목표 이름.
	// 비어 있지 않으면 PR 주소가 없는 것이 정상이다.
	MakeTarget  string
	Files       []string
	Inserted    int
	Skipped     int
	NeedsManual []string
	Err         string
	// 이 기계에 도구가 없어 문법을 확인하지 못한 언어. 확인했다고 침묵하면
	// 받는 사람이 검증된 줄 안다.
	Unverified []string
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

// applyOneRepo 는 한 저장소에 적용하고 PR 을 연다.
// pending 은 아직 배포되지 않은 이름들이다 — 그 이름을 말하는 빌드 오류는
// 우리 탓이 아니다.
func (t *taskContext) applyOneRepo(p insightclient.VariantRepoPlan, req insightclient.VariantPlanRequest, blockers, pending []string) VariantResult {
	r := VariantResult{Repo: p.Repo, NeedsManual: p.NeedsManual}

	if len(p.Changes) == 0 {
		// 넣을 자리가 없는 계획도 온다 — protogen 처럼 생성된 파일만 든
		// 저장소가 그렇다. 사본을 만들 이유가 없다.
		//
		// 그래도 할 일은 남는다: 사람이 채워야 하는 이름, 다시 생성해야 하는
		// 파일, 배포 순서. 그것을 적지 않으면 그 저장소는 아예 없던 일이 된다.
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "VARIANT_NO_SITE",
			fmt.Sprintf("%s 에는 넣을 자리가 없다 — 사람이 할 일 %d개",
				p.Repo, len(p.NeedsManual)+len(p.DepBumps)),
			"", manualWork(p))
		return r
	}

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

	// 우리 편집 전에 빌드가 됐는지 먼저 본다. 안 됐으면 뒤에 견줄 것이 없다.
	builtBefore := GoRepo(repoPath) && len(goBuildErrors(repoPath)) == 0

	out, err := applyVariantPlan(repoPath, p)
	if err != nil {
		r.Err = fmt.Sprintf("적용 실패: %v", err)
		return r
	}
	r.Files, r.Inserted, r.Skipped = out.Files, out.Inserted, out.Skipped

	r.NeedsManual = append(r.NeedsManual, refusalNotes(out.Refused)...)

	if r.Inserted == 0 {
		// 넣은 것이 없으면 PR 을 열 이유가 없다. 다만 까닭이 둘이라 갈라 적는다 —
		// 이미 다 들어 있는 것과, 넣으려다 되돌린 것은 다르다.
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "VARIANT_NOOP",
			fmt.Sprintf("%s 에 넣은 것이 없다 (이미 있음 %d · 되돌림 %d)",
				p.Repo, r.Skipped, len(out.Refused)),
			"", strings.Join(refusalNotes(out.Refused), "\n"))
		return r
	}

	if msg := verifyRepo(repoPath, r.Files, out.Unformatted); msg != "" {
		r.Err = fmt.Sprintf("검증 실패: %s", msg)
		return r
	}
	r.Unverified = unverifiedKinds(r.Files)

	// 문법이 맞아도 뜻이 안 맞는 편집이 있다 — 없는 필드에 값을 더하거나
	// 없는 메서드를 부르는 코드는 파서를 통과한다. 타입까지 빌드로 본다.
	// 원래 빌드가 안 되던 저장소는 우리 탓이 아니므로 보지 않는다.
	var unbuildable []string
	if GoRepo(repoPath) && builtBefore {
		missing, wrong := SplitBuildErrors(UnexpectedBuildErrors(goBuildErrors(repoPath), pending))
		if len(wrong) > 0 {
			r.Err = "빌드가 깨졌다: " + firstLineOf(strings.Join(wrong, "\n"))
			return r
		}
		for _, m := range missing {
			r.NeedsManual = append(r.NeedsManual, "아직 없는 이름이라 빌드가 안 된다: "+m)
		}
		unbuildable = missing
	}

	// **밖으로 나가기 전에 시킨 일인지 본다.**
	//
	// 검증은 "코드가 성립하나" 를 보고, 이것은 "시킨 일인가" 를 본다. 문법이
	// 맞고 빌드도 되는데 시키지 않은 파일을 고치는 것이 가장 위험하다 —
	// 사람은 PR 제목을 보고 통과시킨다. 어긋나면 멈추고 다시 시도하지 않는다.
	if bad := CheckAlignment(AlignmentInput{
		Repo: p.Repo, Value: req.Value, Diff: stagedDiff(repoPath),
		Named: t.namedRepos, Blocker: p.Blocks || p.Publish == protoPublish,
	}); len(bad) > 0 {
		note := AlignmentNote(bad)
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "ALIGNMENT_BLOCKED",
			fmt.Sprintf("%s 의 편집이 요청과 어긋난다 — PR 을 열지 않는다", p.Repo), "", note)
		r.Err = "요청과 어긋나 멈췄다: " + firstLineOf(note)
		r.NeedsManual = append(r.NeedsManual, strings.Split(strings.TrimRight(note, "\n"), "\n")...)
		return r
	}

	msg := variantCommitMessage(req, p)
	url, err := t.orchestrator.gitMgr.PushApprovedChangesOpt(repoPath, p.Repo, branch, msg,
		gitmgr.PushOptions{
			Title:    fmt.Sprintf("%s %s 더한다", p.Repo, korean.With(req.Label, "을", "를")),
			BodyLead: blockerNote(blockers) + unbuildableNote(unbuildable) + steerNote(t.steerNotes),
			// **빌드가 안 되는 PR 은 초안으로 연다.**
			//
			// 없는 이름(새 proto 메시지·필드·메서드가 필요한 것)은 사람이
			// 만들어야 한다. 그런데 그것을 알림으로만 적고 PR 은 보통 PR 로
			// 열었다. 사람은 제목을 보고 통과시킨다 — 머지하면 빌드가 깨진다.
			Draft: len(blockers) > 0 || len(unbuildable) > 0,
		})
	if err != nil {
		r.Err = fmt.Sprintf("PR 을 못 열었다: %v", err)
		r.PRURL = url // 브랜치는 올라갔을 수 있다. 주소를 버리지 않는다.
		return r
	}
	r.PRURL = url
	return r
}

// verifyRepo 는 우리가 고친 파일만 본다.
//
// 저장소 전체를 보면 원래 포맷이 안 맞던 남의 파일 때문에 멀쩡한 PR 이 막힌다
// (worker 의 internal/push/sender.go 가 그랬다).
//
// 함정: gofmt 는 문법 오류가 있으면 종료 코드가 0이 아니다. 그것을 "검증
// 못 함" 으로 보고 넘기면, 잡으라고 만든 경우가 정확히 빠진다.
// 종료 코드가 아니라 출력이 있는지로 판단한다.
func verifyRepo(path string, files, skip []string) string {
	skipSet := map[string]bool{}
	for _, f := range skip {
		skipSet[f] = true
	}
	for _, f := range files {
		if skipSet[f] {
			continue
		}
		switch filepath.Ext(f) {
		case ".go":
			if _, err := exec.LookPath("gofmt"); err != nil {
				continue
			}
			cmd := exec.Command("gofmt", "-e", "-l", f)
			cmd.Dir = path
			b, _ := cmd.CombinedOutput()
			if len(strings.TrimSpace(string(b))) > 0 {
				return "gofmt 가 읽지 못한다: " + firstLineOf(string(b))
			}
		case ".dart":
			if msg := dartParses(filepath.Join(path, f)); msg != "" {
				return msg
			}
		case ".ts", ".js", ".mjs", ".svelte":
			if msg := nodeParses(filepath.Join(path, f)); msg != "" {
				return f + " 를 못 읽는다: " + firstLineOf(msg)
			}
		case ".proto":
			if msg := braceBalance(filepath.Join(path, f)); msg != "" {
				return msg
			}
		}
	}
	return ""
}

// braceBalance 는 중괄호 짝이 맞는지만 본다. protoc 은 여기 없다.
func braceBalance(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	text := string(b)
	if strings.Count(text, "{") != strings.Count(text, "}") {
		return "중괄호가 안 맞는다: " + filepath.Base(path)
	}
	return ""
}

func firstLineOf(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// 확인할 수 있는 언어와, 그 도구가 있어야 확인이 되는 것.
var syntaxTool = map[string]string{
	".go": "gofmt",
}

// unverifiedKinds 는 고친 파일 가운데 문법을 확인하지 못한 확장자를 준다.
func unverifiedKinds(files []string) []string {
	var out []string
	for _, f := range files {
		ext := filepath.Ext(f)
		if ext == ".dart" && dartBin() == "" {
			out = appendOnceStr(out, ".dart")
			continue
		}
		if nodeParsedExts[ext] && tsParserScript() == "" {
			out = appendOnceStr(out, ext)
			continue
		}
		tool, ok := syntaxTool[ext]
		if !ok {
			continue
		}
		if _, err := exec.LookPath(tool); err == nil {
			continue
		}
		out = appendOnceStr(out, ext)
	}
	sort.Strings(out)
	return out
}

func appendOnceStr(xs []string, x string) []string {
	for _, v := range xs {
		if v == x {
			return xs
		}
	}
	return append(xs, x)
}

// refusalNotes 는 되돌린 자리를 사람이 읽을 줄로 만든다.
// 조용히 빼면 무엇이 빠졌는지 알 수 없다.
func refusalNotes(refused []RefusedChange) []string {
	var out []string
	for _, x := range refused {
		out = append(out, fmt.Sprintf("%s:%d — 넣으면 문법이 깨져 되돌렸다: %s",
			x.File, x.Line, x.Why))
	}
	return out
}
