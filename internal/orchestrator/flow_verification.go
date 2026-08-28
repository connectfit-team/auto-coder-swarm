package orchestrator

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "HEALING_DIAGNOSIS", fmt.Sprintf("빌드 실패, 자가 치유 가동 (%d/3)", healAttempt), string(buildOut), "")

		relevantFiles := make(map[string]string)
		plan := t.ctx.Value("current_plan").(agent.Plan)
		for _, change := range plan.Changes {
			content, _ := os.ReadFile(filepath.Join(t.repoPath, change.FilePath))
			relevantFiles[change.FilePath] = string(content)
		}

		healingPlan, hErr := t.healer.ProposeHealing(t.ctx, string(buildOut), t.meta.Type, relevantFiles)
		if hErr != nil {
			t.lastFeedback = fmt.Sprintf("HEALER LLM CRASHED: %v\nBUILD ERROR:\n%s", hErr, string(buildOut))
			exec.CommandContext(t.ctx, "git", "-C", t.repoPath, "checkout", ".").Run()
			return false, nil
		}

		abort := false
		for _, step := range healingPlan.Steps {
			switch step.Action {
			case healing.ActionModifyCode:
				t.coder.ModifyFile(t.ctx, filepath.Join(t.repoPath, step.TargetFile), step.Instruction)
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

	// 프롬프트에 절차를 넣어도 지키지 않는 일이 있다. 검사할 수 있는 것은
	// 모델의 판단에 맡기지 않고 기계로 본다.
	if v := checkProcedureViolations(t.finalDiff); len(v) > 0 {
		t.lastFeedback = "PROCEDURE VIOLATION: " + strings.Join(v, " / ")
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "SKILL_VIOLATION", "작업 절차 위반", t.lastFeedback, "")
		exec.CommandContext(t.ctx, "git", "-C", t.repoPath, "checkout", ".").Run()
		return false, RunResult{}, nil
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
	if !t.critic.IsApproved(criticResp) {
		// 거절 이유를 남긴다. 안 남기면 왜 다시 도는지 밖에서 알 수 없다 —
		// 로그에는 계획→코딩→계획 만 반복해 찍힌다.
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CRITIC_REJECT", "비평가가 거절했다", "", criticResp)
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
	if !t.reviewer.IsApproved(reviewResp) {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "REVIEWER_REJECT", "리뷰어가 거절했다", "", reviewResp)
		t.lastFeedback = "REVIEWER REJECTION: " + reviewResp
		exec.CommandContext(t.ctx, "git", "-C", t.repoPath, "checkout", ".").Run()
		return false, RunResult{}, nil
	}

	if !t.isApproved {
		return true, RunResult{RepoName: t.targetRepo, WaitingApproval: true}, nil
	}

	prURL, _ := t.orchestrator.gitMgr.PushApprovedChanges(t.repoPath, t.targetRepo, t.currentBranch, "feat: automated enhancement")
	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "COMPLETED", "성공", prURL, "")
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
