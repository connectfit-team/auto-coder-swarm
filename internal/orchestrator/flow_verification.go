package orchestrator

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
	"github.com/connectfit-team/auto-coder-swarm/internal/healing"
)

func (t *taskContext) stepVerification() (bool, error) {
	for healAttempt := 1; healAttempt <= 3; healAttempt++ {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "BUILD", fmt.Sprintf("[%s] 검증 (%s) - 시도 %d", t.meta.Type, t.meta.BuildCommand, healAttempt), t.meta.BuildCommand, "")
		bCmd := shellCmd(t.ctx, t.repoPath, t.meta.BuildCommand)
		buildOut, err := bCmd.CombinedOutput()

		// 빌드가 통과했으면 **바뀐 패키지의 테스트를 실제로 돌린다.**
		// 컴파일만 보면 틀린 테스트가 통과한다 — 테스트를 쓰는 것이 일의
		// 절반인데 그게 도는지 확인하지 않으면 절반은 검증되지 않은 셈이다.
		if err == nil {
			if out, ok := t.runChangedTests(t.ctx.Value("current_plan").(agent.Plan)); !ok {
				t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "TEST_FAIL", "바뀐 패키지의 테스트가 실패했다", "", clip(out, 1500))
				buildOut = []byte(out)
				err = errChangedTestsFailed
			}
		}

		if err == nil {
			// Build succeeded
			if t.meta.BenchCommand != "" {
				cmd := shellCmd(t.ctx, t.repoPath, t.meta.BenchCommand)
				bOut, _ := cmd.CombinedOutput()
				t.postBench = string(bOut)
			}
			return true, nil
		}

		if t.ctx.Err() != nil {
			return false, t.ctx.Err()
		}

		// **경고를 걷어내고 오류만 넘긴다.**
		//
		// vite 는 손대지도 않은 파일의 A11y 경고를 앞에 잔뜩 찍는다. 그 앞부분이
		// 그대로 치유기에 들어가서, 8,192 토큰이 경고로 차고 진짜 오류는 잘려
		// 나갔다. 기준 빌드는 같은 경고를 달고도 통과했으니 그건 원인이 아니다.
		failure := distillBuildError(string(buildOut))
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "HEALING_DIAGNOSIS", fmt.Sprintf("빌드 실패, 자가 치유 가동 (%d/3)", healAttempt), failure, "")

		relevantFiles := make(map[string]string)
		plan := t.ctx.Value("current_plan").(agent.Plan)
		for _, change := range plan.Changes {
			content, _ := os.ReadFile(filepath.Join(t.repoPath, change.FilePath))
			relevantFiles[change.FilePath] = string(content)
		}

		healingPlan, hErr := t.healer.ProposeHealing(t.ctx, failure, t.meta.Type, relevantFiles)
		if hErr != nil {
			t.lastFeedback = fmt.Sprintf("HEALER LLM CRASHED: %v\nBUILD ERROR:\n%s", hErr, failure)
			exec.CommandContext(t.ctx, "git", "-C", t.repoPath, "checkout", ".").Run()
			return false, nil
		}

		abort := false
		for _, step := range healingPlan.Steps {
			switch step.Action {
			case healing.ActionModifyCode:
				if _, err := t.coder.ModifyFile(t.ctx, filepath.Join(t.repoPath, step.TargetFile), step.Instruction); err != nil {
					t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "HEALING_FAILED",
						fmt.Sprintf("[%s] 치유가 파일을 못 고쳤습니다", step.TargetFile), "", err.Error())
				}
			case healing.ActionRunCommand:
				shellCmd(t.ctx, t.repoPath, step.Command).Run()
			case healing.ActionAbort:
				abort = true
			}
		}

		if abort {
			t.lastFeedback = fmt.Sprintf("HEALER ABORTED: %s\nBUILD ERROR:\n%s", healingPlan.Diagnosis, string(buildOut))
			exec.CommandContext(t.ctx, "git", "-C", t.repoPath, "checkout", ".").Run()
			return false, nil
		}
	}

	// Exhausted all heal attempts
	t.lastFeedback = "HEALER FAILED TO FIX BUILD AFTER 3 ATTEMPTS"
	exec.CommandContext(t.ctx, "git", "-C", t.repoPath, "checkout", ".").Run()
	return false, nil
}

func (t *taskContext) stepReview() (bool, RunResult, error) {
	diffCmd := exec.CommandContext(t.ctx, "git", "-C", t.repoPath, "diff", "HEAD")
	diffOut, _ := diffCmd.CombinedOutput()
	t.finalDiff = string(diffOut)
	t.orchestrator.store.UpdateTaskProposedDiff(t.taskID, t.finalDiff)

	// **파일을 통째로 지우는 것은 고침이 아니다.**
	//
	// "말일 데이터가 빠진다" 를 고치라고 했더니 prisma/logout.schema.prisma
	// 1,065줄을 지운 diff 가 승인 대기까지 왔다. 코더가 "수정" 을 "다시 씀"
	// 으로 읽으면 원본을 날린다. 지운 줄이 압도적으로 많으면 사람 손에
	// 넘기기 전에 여기서 막는다.
	if wiped := findWipedFiles(t.ctx, t.repoPath); len(wiped) > 0 {
		t.lastFeedback = "DESTRUCTIVE DIFF: 파일을 거의 통째로 지웠다 — " + strings.Join(wiped, " / ") +
			"\n고칠 줄만 바꿔라. 파일을 새로 쓰지 마라."
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "DESTRUCTIVE_DIFF",
			"파일을 거의 통째로 지우는 변경이라 되돌렸습니다", "", strings.Join(wiped, "\n"))
		exec.CommandContext(t.ctx, "git", "-C", t.repoPath, "checkout", ".").Run()
		return false, RunResult{}, nil
	}

	// 프롬프트에 절차를 넣어도 지키지 않는 일이 있다. 검사할 수 있는 것은
	// 모델의 판단에 맡기지 않고 기계로 본다.
	if v := checkProcedureViolations(t.finalDiff); len(v) > 0 {
		t.lastFeedback = "PROCEDURE VIOLATION: " + strings.Join(v, " / ")
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "SKILL_VIOLATION", "작업 절차 위반", t.lastFeedback, "")
		exec.CommandContext(t.ctx, "git", "-C", t.repoPath, "checkout", ".").Run()
		return false, RunResult{}, nil
	}

	// **바뀐 것이 없으면 승인할 것도 없다.**
	//
	// 빈 diff 는 검토 두 관문을 그냥 통과한다(지적할 자리가 없으니까).
	// 그래서 아무것도 안 한 작업이 "성공" 으로 기록됐다.
	if strings.TrimSpace(t.finalDiff) == "" {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "EMPTY_DIFF", "바뀐 것이 없다", "", "")
		return false, RunResult{}, fmt.Errorf("코드가 하나도 바뀌지 않았다 — 승인할 것이 없다")
	}

	// **줄 끝 공백만 바뀐 것도 안 바뀐 것이다.**
	//
	// 실측으로 diff 11줄이 올라왔는데 내용은 파일 끝 개행 하나였다
	// ("\ No newline at end of file"). 빈 diff 검사는 통과하고, 검토 관문은
	// 지적할 자리가 없어 통과시킨다 — 아무것도 안 한 작업이 승인 대기까지 갔다.
	// **주석만 바꾼 것도 고친 것이 아니다.**
	//
	// 실측으로 감사 로그 주석 한 줄에 "이 방법을 따지세요" 를 덧붙인 diff 가
	// 승인 대기까지 왔다. 공백 검사에는 안 걸린다 — 글자가 실제로 바뀌었으니까.
	// 주석을 고쳐 달라고 한 작업이면 그건 맞는 결과이므로 요청문을 함께 본다.
	if !wantsCommentWork(t.req.UserRequest) && isCommentOnlyDiff(t.finalDiff) {
		t.lastFeedback = "COMMENT ONLY: 주석만 바꿨다. 동작을 고쳐라 — 조건식·비교·경계값처럼 실제로 도는 코드를 바꿔야 한다."
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "COMMENT_ONLY_DIFF",
			"주석만 바뀌었다 — 동작이 그대로다", "", clip(t.finalDiff, 800))
		exec.CommandContext(t.ctx, "git", "-C", t.repoPath, "checkout", ".").Run()
		return false, RunResult{}, nil
	}

	if isCosmeticDiff(t.finalDiff) {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "COSMETIC_DIFF",
			"공백만 바뀌었다 — 실제로 고친 것이 없다", "", clip(t.finalDiff, 800))
		exec.CommandContext(t.ctx, "git", "-C", t.repoPath, "checkout", ".").Run()
		return false, RunResult{}, fmt.Errorf("공백만 바뀌었다 — 승인할 것이 없다")
	}

	securityFindings, _ := t.orchestrator.securityGuard.ExecuteAll(t.ctx, t.repoPath, t.finalDiff)
	var securityFeedback strings.Builder
	if len(securityFindings) > 0 {
		securityFeedback.WriteString("\n[SECURITY FINDINGS]\n")
		for _, f := range securityFindings {
			securityFeedback.WriteString(fmt.Sprintf("- [%s] %s\n", f.Level, f.Message))
		}
	}

	// 컨텍스트가 8,192 토큰이다. diff 와 벤치 출력이 그걸 넘기면 모델이
	// 빈 응답을 낸다 — 그게 거절로 읽혀 작업이 통째로 버려졌다.
	reviewInput := fmt.Sprintf("DIFF:\n%s\n\nPRE-BENCH:\n%s\n\nPOST-BENCH:\n%s%s",
		clip(t.finalDiff, 6000), clip(t.preBench, 800), clip(t.postBench, 800), securityFeedback.String())

	// **리뷰 호출 자체가 실패한 것과 거절을 구분한다.**
	//
	// 전에는 오류를 `_` 로 버렸다. 모델이 아무것도 못 내면 빈 문자열이 오고,
	// IsApproved("") 는 false 라 **거절로 읽힌다.** 그래서 세 번을 다시 돌고
	// "최대 시도 초과" 로 죽었는데, 로그에는 이유가 한 줄도 없었다.
	criticResp, cErr := t.critic.Process(t.ctx, reviewInput)
	if cErr != nil {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "REVIEW_ERROR", "비평가 호출 실패", "", cErr.Error())
		return false, RunResult{}, fmt.Errorf("비평가를 부르지 못했다: %w", cErr)
	}
	if strings.TrimSpace(criticResp) == "" {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "REVIEW_ERROR", "비평가가 빈 응답을 냈다", reviewInput[:min(len(reviewInput), 400)], "")
		return false, RunResult{}, fmt.Errorf("비평가가 빈 응답을 냈다 (입력 %d자 — 컨텍스트를 넘겼을 수 있다)", len(reviewInput))
	}
	// **막연한 걱정으로 막지 않는다.**
	//
	// "위험을 찾아라" 라고만 시키면 9B 는 늘 찾아낸다. 실측으로 enum 에
	// String() 을 더한 변경이 "Unknown 을 돌려주면 민감정보가 샐 수 있다" 로
	// 거절됐다. 그것이 세 번 반복되면 작업은 통째로 버려진다.
	// 파일·줄을 못 대면 위험이 아니다.
	cv := agent.ParseCriticVerdict(criticResp, t.finalDiff)
	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CRITIC", cv.Why, "", criticResp)
	if cv.Blocking {
		t.lastFeedback = "CRITIC REJECTION: " + criticResp
		exec.CommandContext(t.ctx, "git", "-C", t.repoPath, "checkout", ".").Run()
		return false, RunResult{}, nil
	}

	reviewResp, rErr := t.reviewer.Process(t.ctx, reviewInput)
	if rErr != nil {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "REVIEW_ERROR", "리뷰어 호출 실패", "", rErr.Error())
		return false, RunResult{}, fmt.Errorf("리뷰어를 부르지 못했다: %w", rErr)
	}
	if strings.TrimSpace(reviewResp) == "" {
		return false, RunResult{}, fmt.Errorf("리뷰어가 빈 응답을 냈다 (입력 %d자)", len(reviewInput))
	}
	rv := agent.ParseReviewerVerdict(reviewResp, t.finalDiff)
	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "REVIEWER", rv.Why, "", reviewResp)
	if rv.Blocking {
		t.lastFeedback = "REVIEWER REJECTION: " + reviewResp
		exec.CommandContext(t.ctx, "git", "-C", t.repoPath, "checkout", ".").Run()
		return false, RunResult{}, nil
	}

	if !t.isApproved {
		return true, RunResult{RepoName: t.targetRepo, WaitingApproval: true}, nil
	}

	// **커밋 메시지가 "feat: automated enhancement" 였다.**
	//
	// 무엇을 왜 바꿨는지 아무것도 안 남으므로, 나중에 이 커밋을 만난 사람이
	// 다시 diff 를 읽어야 한다. 요청한 말을 그대로 쓴다.
	prURL, prErr := t.orchestrator.gitMgr.PushApprovedChanges(
		t.repoPath, t.targetRepo, t.currentBranch, commitMessageFor(t.req.UserRequest))
	if prErr != nil {
		// PR 을 못 열어도 브랜치는 올라가 있다. 그 주소를 남긴다 —
		// 버리면 사람은 브랜치 이름조차 못 듣는다.
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "PR_MANUAL",
			"PR 은 못 열었지만 브랜치는 올라갔습니다", prURL, prErr.Error())
	} else {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "COMPLETED", "성공", prURL, "")
	}
	return true, RunResult{RepoName: t.targetRepo, PRURL: prURL}, nil
}

// clip 은 UTF-8 경계에서 자르고 잘렸다는 것을 알린다.
func clip(s string, max int) string {
	if len(s) <= max {
		return s
	}
	s = s[:max]
	for len(s) > 0 && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s + "\n… (이하 생략)"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// 테스트 실패를 빌드 실패와 같은 길로 보낸다 — 자가치유가 고칠 대상이다.
var errChangedTestsFailed = &changedTestsError{}

type changedTestsError struct{}

func (*changedTestsError) Error() string { return "바뀐 패키지의 테스트가 실패했다" }

// 빌드 출력에서 오류로 보이는 줄.
var buildErrorMarkers = []string{
	"error", "err!", "failed", "failure", "cannot find", "not found",
	"unexpected", "is not assignable", "does not exist", "syntaxerror",
	"typeerror", "referenceerror", "missing", "expected",
}

// 경고일 뿐인 줄. 기준 빌드도 이것을 달고 통과한다.
var buildNoiseMarkers = []string{
	"a11y:", "warning", "warn ", "deprecated", "npm notice",
	"browserslist", "vite v", "building ", "transforming",
}

// distillBuildError 는 빌드 출력에서 고칠 거리가 되는 줄만 남긴다.
//
// 앞에서 자르면 안 된다 — 빌드 도구는 경고를 먼저, 오류를 나중에 찍는다.
// 아무것도 못 고르면 앞이 아니라 **뒤**를 준다.
func distillBuildError(out string) string {
	const maxLines = 40
	lines := strings.Split(out, "\n")

	var kept []string
	for i, ln := range lines {
		low := strings.ToLower(ln)
		noisy := false
		for _, m := range buildNoiseMarkers {
			if strings.Contains(low, m) {
				noisy = true
				break
			}
		}
		if noisy {
			continue
		}
		for _, m := range buildErrorMarkers {
			if strings.Contains(low, m) {
				kept = append(kept, ln)
				// 오류 줄 바로 다음 두 줄에 파일·위치가 붙는다.
				for j := i + 1; j < len(lines) && j <= i+2; j++ {
					if strings.TrimSpace(lines[j]) != "" {
						kept = append(kept, lines[j])
					}
				}
				break
			}
		}
		if len(kept) >= maxLines {
			break
		}
	}

	if len(kept) == 0 {
		// 고를 것이 없으면 끝부분을 준다. 오류는 대개 마지막에 있다.
		tail := lines
		if len(tail) > maxLines {
			tail = tail[len(tail)-maxLines:]
		}
		return strings.TrimSpace(strings.Join(tail, "\n"))
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}

// 이만큼 지워지면 "고친" 것이 아니라 "날린" 것으로 본다.
const (
	wipeMinDeleted = 50 // 이보다 적게 지웠으면 정상적인 정리일 수 있다
	wipeRatio      = 5  // 지운 줄이 더한 줄의 이 배를 넘으면 의심한다
)

// findWipedFiles 는 거의 통째로 지워진 파일을 찾는다.
//
// git diff --numstat 이 파일마다 "더한 줄\t지운 줄\t경로" 를 준다. 파일을
// 통째로 지우는 것이 목적인 작업도 있으므로, 지운 양이 크고 더한 양이
// 하찮을 때만 막는다. 이진 파일은 숫자 대신 "-" 가 와서 저절로 걸러진다.
func findWipedFiles(ctx context.Context, repoPath string) []string {
	out, err := exec.CommandContext(ctx, "git", "-C", repoPath, "diff", "--numstat", "HEAD").Output()
	if err != nil {
		return nil
	}
	var wiped []string
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Split(strings.TrimSpace(line), "\t")
		if len(f) < 3 {
			continue
		}
		added, err1 := strconv.Atoi(f[0])
		deleted, err2 := strconv.Atoi(f[1])
		if err1 != nil || err2 != nil {
			continue
		}
		if deleted >= wipeMinDeleted && deleted >= added*wipeRatio {
			wiped = append(wiped, fmt.Sprintf("%s (+%d/-%d)", f[2], added, deleted))
		}
	}
	return wiped
}

// isCosmeticDiff 는 내용이 실제로 바뀌었는지 본다.
//
// 더한 줄과 뺀 줄에서 공백을 지운 것이 서로 같으면 바뀐 것이 없다. 줄 끝
// 개행, 들여쓰기, 줄바꿈만 바뀐 diff 가 여기에 걸린다.
func isCosmeticDiff(diff string) bool {
	added := map[string]int{}
	removed := map[string]int{}
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"):
			continue
		case strings.HasPrefix(line, "+"):
			if t := strings.Join(strings.Fields(line[1:]), " "); t != "" {
				added[t]++
			}
		case strings.HasPrefix(line, "-"):
			if t := strings.Join(strings.Fields(line[1:]), " "); t != "" {
				removed[t]++
			}
		}
	}
	if len(added) != len(removed) {
		return false
	}
	for k, n := range added {
		if removed[k] != n {
			return false
		}
	}
	return true
}

// 주석 한 줄의 시작 표시. 언어를 가리지 않고 흔한 것만 본다.
var commentPrefixes = []string{"//", "/*", "*/", "*", "#", "<!--", "-->", "--"}

func isCommentLine(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return true
	}
	for _, p := range commentPrefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// isCommentOnlyDiff 는 바뀐 줄이 전부 주석인지 본다.
func isCommentOnlyDiff(diff string) bool {
	changed := 0
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") {
			continue
		}
		if !strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "-") {
			continue
		}
		body := line[1:]
		if strings.TrimSpace(body) == "" {
			continue
		}
		changed++
		if !isCommentLine(body) {
			return false
		}
	}
	return changed > 0
}

// wantsCommentWork 는 요청이 주석·문서 작업인지 본다.
func wantsCommentWork(req string) bool {
	low := strings.ToLower(req)
	for _, w := range []string{"주석", "comment", "문서", "docs", "doc comment", "godoc", "jsdoc"} {
		if strings.Contains(low, w) {
			return true
		}
	}
	return false
}

// commitMessageFor 는 요청문에서 커밋 제목을 만든다.
//
// 첫 문장만 쓰고 너무 길면 자른다. 요청이 비면 예전 문구로 물러선다.
func commitMessageFor(request string) string {
	r := strings.TrimSpace(request)
	if r == "" {
		return "fix: 자동 수정"
	}
	for _, sep := range []string{"\n", ". ", "다. ", "요. "} {
		if i := strings.Index(r, sep); i > 10 {
			r = r[:i+len(sep)-1]
			break
		}
	}
	r = strings.TrimSpace(strings.Trim(r, ".。 "))
	if n := []rune(r); len(n) > 60 {
		r = string(n[:60]) + "…"
	}
	return "fix: " + r
}
