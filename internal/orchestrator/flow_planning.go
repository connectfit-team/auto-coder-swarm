package orchestrator

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"
)

func (t *taskContext) stepPlanning(attempt int) error {
	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "PLANNING", fmt.Sprintf("계획 수립 (시도 %d/3)", attempt), t.analysis, "")
	
	// Inject Analysis AND CKH Knowledge into Planning
	input := t.analysis
	if t.ckhKnowledge != "" {
		input = fmt.Sprintf("[CORPORATE KNOWLEDGE]\n%s\n\n[CODE ANALYSIS]\n%s", t.ckhKnowledge, input)
	}

	if t.lastFeedback != "" {
		input += "\n\nFEEDBACK:\n" + t.lastFeedback
	}

	voteRes, _ := t.voter.Vote(t.ctx, "Planner", t.planner.BuildPrompt(input))
	plan, err := t.planner.ParsePlan(voteRes.Winner)
	if err != nil { return err }

	if attempt == 1 {
		t.targetRepo = plan.RepoName
		if t.req.TargetRepo != "" { t.targetRepo = t.req.TargetRepo }
		if t.repoLockFunc != nil { t.repoLockFunc(t.targetRepo) }

		t.repoPath = filepath.Join(t.wsPath, "repo")
		t.currentBranch = fmt.Sprintf("acs-fix-%s", time.Now().Format("0102150405"))
		t.orchestrator.wsMgr.CreateWorktree(t.targetRepo, t.repoPath, t.currentBranch)
		t.meta = t.orchestrator.detectProjectTypeLLM(t.ctx, t.taskID, t.repoPath)

		if t.meta.BenchCommand != "" {
			cmd := exec.CommandContext(t.ctx, "bash", "-c", t.meta.BenchCommand)
			cmd.Dir = t.repoPath
			bOut, _ := cmd.CombinedOutput()
			t.preBench = string(bOut)
		}
	}

	t.orchestrator.store.AddLog(t.taskID, "PLAN", fmt.Sprintf("파일 %d개 수정 계획 수립", len(plan.Changes)))
	t.ctx = context.WithValue(t.ctx, "current_plan", plan)
	return nil
}
