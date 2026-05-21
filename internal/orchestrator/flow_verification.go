package orchestrator

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
	"github.com/connectfit-team/auto-coder-swarm/internal/healing"
)

func (t *taskContext) stepVerification() (bool, error) {
	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "BUILD", fmt.Sprintf("[%s] 검증 (%s)", t.meta.Type, t.meta.BuildCommand), t.meta.BuildCommand, "")
	bCmd := exec.CommandContext(t.ctx, "bash", "-c", t.meta.BuildCommand)
	bCmd.Dir = t.repoPath
	buildOut, err := bCmd.CombinedOutput()

	if err != nil {
		if t.ctx.Err() != nil {
			return false, t.ctx.Err()
		}
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "HEALING_DIAGNOSIS", "빌드 실패, 자가 치유 가동", string(buildOut), "")

		relevantFiles := make(map[string]string)
		plan := t.ctx.Value("current_plan").(agent.Plan)
		for _, change := range plan.Changes {
			content, _ := os.ReadFile(filepath.Join(t.repoPath, change.FilePath))
			relevantFiles[change.FilePath] = string(content)
		}

		healingPlan, hErr := t.healer.ProposeHealing(t.ctx, string(buildOut), t.meta.Type, relevantFiles)
		if hErr != nil {
			t.lastFeedback = fmt.Sprintf("BUILD FAILED: %v", err)
			exec.CommandContext(t.ctx, "git", "-C", t.repoPath, "checkout", ".").Run()
			return false, nil
		}

		abort := false
		for _, step := range healingPlan.Steps {
			switch step.Action {
			case healing.ActionModifyCode:
				t.coder.ModifyFile(t.ctx, filepath.Join(t.repoPath, step.TargetFile), step.Instruction)
			case healing.ActionRunCommand:
				exec.CommandContext(t.ctx, "bash", "-c", step.Command).Run()
			case healing.ActionAbort:
				abort = true
			}
		}
		if abort {
			return false, fmt.Errorf("Healer aborted: %s", healingPlan.Diagnosis)
		}
		t.lastFeedback = "HEALING ATTEMPTED"
		return false, nil
	}

	if t.meta.BenchCommand != "" {
		cmd := exec.CommandContext(t.ctx, "bash", "-c", t.meta.BenchCommand)
		cmd.Dir = t.repoPath
		bOut, _ := cmd.CombinedOutput()
		t.postBench = string(bOut)
	}
	return true, nil
}

func (t *taskContext) stepReview() (bool, RunResult, error) {
	diffCmd := exec.CommandContext(t.ctx, "git", "-C", t.repoPath, "diff", "HEAD")
	diffOut, _ := diffCmd.CombinedOutput()
	t.finalDiff = string(diffOut)
	t.orchestrator.store.UpdateTaskProposedDiff(t.taskID, t.finalDiff)

	securityFindings, _ := t.orchestrator.securityGuard.ExecuteAll(t.ctx, t.repoPath, t.finalDiff)
	var securityFeedback strings.Builder
	if len(securityFindings) > 0 {
		securityFeedback.WriteString("\n[SECURITY FINDINGS]\n")
		for _, f := range securityFindings {
			securityFeedback.WriteString(fmt.Sprintf("- [%s] %s\n", f.Level, f.Message))
		}
	}

	reviewInput := fmt.Sprintf("DIFF:\n%s\n\nPRE-BENCH:\n%s\n\nPOST-BENCH:\n%s%s",
		t.finalDiff, t.preBench, t.postBench, securityFeedback.String())

	criticResp, _ := t.critic.Process(t.ctx, reviewInput)
	if !t.critic.IsApproved(criticResp) {
		t.lastFeedback = "CRITIC REJECTION: " + criticResp
		exec.CommandContext(t.ctx, "git", "-C", t.repoPath, "checkout", ".").Run()
		return false, RunResult{}, nil
	}

	reviewResp, _ := t.reviewer.Process(t.ctx, reviewInput)
	if !t.reviewer.IsApproved(reviewResp) {
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
