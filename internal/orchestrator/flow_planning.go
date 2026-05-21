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
	input := t.analysis
	if t.lastFeedback != "" {
		input += "\n\nFEEDBACK:\n" + t.lastFeedback
	}
	voteRes, _ := t.voter.Vote(t.ctx, "Planner", t.planner.BuildPrompt(input))
	plan, err := t.planner.ParsePlan(voteRes.Winner)
	if err != nil {
		return err
	}

	if attempt == 1 {
		t.targetRepo = plan.RepoName
		if t.req.TargetRepo != "" {
			t.targetRepo = t.req.TargetRepo
		}
		if t.repoLockFunc != nil {
			t.repoLockFunc(t.targetRepo)
		}

		t.repoPath = filepath.Join(t.wsPath, "repo")
		t.currentBranch = fmt.Sprintf("swarm-fix-%s", time.Now().Format("0102150405"))
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
	// We need to keep the actual plan changes for the execution step
	t.ctx = context.WithValue(t.ctx, "current_plan", plan)
	return nil
}
