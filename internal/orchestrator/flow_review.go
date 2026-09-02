package orchestrator

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
)

func (t *taskContext) stepReview() (bool, RunResult, error) {
	// **계획에 없는 파일이 섞였으면 그 파일만 되돌린다.**
	//
	// 작업 전체를 버릴 일은 아니다 — 맞게 고친 파일은 살리고, 요청하지 않은
	// 파일만 원래대로 되돌린다. 실측으로 말일 경계 수정에 export API 변경이
	// 딸려 와 승인 대기까지 갔다.
	if plan, ok := t.ctx.Value("current_plan").(agent.Plan); ok {
		if reverted := t.revertUnplannedFiles(plan); len(reverted) > 0 {
			t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "UNPLANNED_REVERTED",
				fmt.Sprintf("계획에 없는 파일 %d개를 되돌렸습니다", len(reverted)),
				"", strings.Join(reverted, "\n"))
		}
	}

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
		// 분석이 짚은 파일에 고칠 것이 없었다면 그 후보가 틀린 것이다.
		// 남은 시도를 같은 입력에 쓰지 말고 다음 후보로 넘긴다.
		if paths := t.markAnalysisDeadEnd(); len(paths) > 0 {
			t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "ANALYSIS_DEAD_END",
				"분석이 짚은 곳에 고칠 것이 없어 다음 후보로 넘어갑니다", "", strings.Join(paths, "\n"))
			return false, RunResult{}, nil
		}
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

	// 검토자에게 요청문과 분석을 함께 준다 — 근거 없이 diff 만 보면
	// 맞는 수정을 되돌리라고 한다.
	reviewResp, rErr := t.reviewer.ProcessWithContext(t.ctx, reviewInput, t.req.UserRequest, t.analysis)
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
		// **분석이 처방한 수정이면 검토자의 반대는 자문이다.**
		//
		// 검토자가 경계값을 못 읽는다. 실측으로 말일 경계를 정확히 고친
		// 최소 수정(파일 1개)을 세 번 연속 반려하면서 "lt 를 다음 달 1일로
		// 옮기면 말일이 빠진다" 고 했다 — 정확히 거꾸로다. 그 말을 따르면
		// 원래 버그가 그대로 남는다.
		//
		// 근거는 분석 쪽에 있다(원인 줄·이유·고칠 값). 그러니 되돌리지 않고
		// 반대 의견을 붙여 사람에게 넘긴다. 사람이 보면 5초면 판정할 일이다.
		if t.planFromAnalysis {
			t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "REVIEW_ADVISORY",
				"검토자가 반대했지만 분석이 짚은 수정이라 사람 판단으로 넘깁니다", "", reviewResp)
			return true, RunResult{RepoName: t.targetRepo, WaitingApproval: true}, nil
		}
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
