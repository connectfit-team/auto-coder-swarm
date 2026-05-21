package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
	"github.com/connectfit-team/auto-coder-swarm/internal/healing"
	"github.com/connectfit-team/auto-coder-swarm/internal/observability"
)

func (t *taskContext) execute() (RunResult, error) {
	observability.ActiveWorkers.Inc()
	defer observability.ActiveWorkers.Dec()

	start := time.Now()
	if err := t.prepareAnalysis(); err != nil {
		return RunResult{}, err
	}
	observability.RecordStepDuration("analysis", t.targetRepo, time.Since(start).Seconds())

	wsPath, err := t.orchestrator.wsMgr.CreateWorkspace()
	if err != nil {
		return RunResult{}, err
	}
	t.wsPath = wsPath
	defer t.orchestrator.wsMgr.Cleanup(wsPath)

	for attempt := 1; attempt <= 3; attempt++ {
		if t.ctx.Err() != nil {
			return RunResult{}, t.ctx.Err()
		}

		startPlan := time.Now()
		if err := t.stepPlanning(attempt); err != nil {
			observability.IncrementAgentOp("Planner", "failed")
			return RunResult{}, err
		}
		observability.RecordStepDuration("planning", t.targetRepo, time.Since(startPlan).Seconds())
		observability.IncrementAgentOp("Planner", "success")

		startExec := time.Now()
		if err := t.stepExecution(attempt); err != nil {
			observability.IncrementAgentOp("Coder", "failed")
			return RunResult{}, err
		}
		observability.RecordStepDuration("execution", t.targetRepo, time.Since(startExec).Seconds())
		observability.IncrementAgentOp("Coder", "success")

		startVerif := time.Now()
		success, err := t.stepVerification()
		if err != nil {
			observability.RecordStepDuration("verification", t.targetRepo, time.Since(startVerif).Seconds())
			return RunResult{}, err
		}
		observability.RecordStepDuration("verification", t.targetRepo, time.Since(startVerif).Seconds())
		if !success {
			continue
		}

		startReview := time.Now()
		finished, res, err := t.stepReview()
		if err != nil {
			observability.RecordStepDuration("review", t.targetRepo, time.Since(startReview).Seconds())
			return RunResult{}, err
		}
		observability.RecordStepDuration("review", t.targetRepo, time.Since(startReview).Seconds())
		if finished {
			return res, nil
		}
	}

	return RunResult{RepoName: t.targetRepo}, fmt.Errorf("최대 시도 초과")
}

func (t *taskContext) prepareAnalysis() error {
	if t.analysis != "" {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "INIT", "분석 컨텍스트 주입됨", "", t.analysis)
		return nil
	}

	sessionID := fmt.Sprintf("swarm-task-%s", t.taskID)

	// Inspection
	inspectionPrompt := fmt.Sprintf("[User Request]\n%s\n\n위 요청 범위 내에 있는 모든 대상 파일들의 '정확한 경로 목록'과 각 파일의 '코드 라인 수(LOC)'를 알려줘. 분석은 하지 말고 목록과 수량만 라이트하게 응답해라.", t.req.UserRequest)
	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "INSPECTION", "사전 검사 요청", inspectionPrompt, "")

	inspectRes, err := t.orchestrator.insightClient.QueryOracle(t.ctx, inspectionPrompt, sessionID)
	if err != nil {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "ERROR", "사전 검사 실패", err.Error(), "")
		return err
	}

	// Strategy
	strategyPrompt := fmt.Sprintf(`사전 검사 결과를 바탕으로 작업 전략을 수립해줘. 
CIE(Eyes)에게는 '코드의 논리적 이해'와 '기술적 요약'만 요청해야 한다.

[User Request]
%s

[Inspection Result]
%s

MANDATORY JSON FORMAT:
{
  "total_files": 0,
  "total_lines": 0,
  "complexity_risk": "...",
  "actionable_path": ["..."],
  "analysis_query": "...",
  "is_feasible": true
}
`, t.req.UserRequest, inspectRes)

	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "STRATEGY", "전략 수립 중", strategyPrompt, "")
	stratRaw, _ := agent.CallLLM(t.ctx, t.primaryLLM, "Architect", strategyPrompt)

	var strategy TaskStrategy
	json.Unmarshal([]byte(extractJSON(stratRaw)), &strategy)

	if !strategy.IsFeasible {
		return fmt.Errorf("작업 규모 과다 (%d 파일)", strategy.TotalFiles)
	}

	// Precision Analysis
	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "ORACLE", "CIE 정밀 분석 요청", strategy.AnalysisQuery, "")
	res, err := t.orchestrator.insightClient.QueryOracle(t.ctx, strategy.AnalysisQuery, sessionID)
	if err != nil {
		return err
	}
	t.analysis = res
	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "INIT", "분석 완료", "", t.analysis)
	return nil
}

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

func (t *taskContext) stepExecution(attempt int) error {
	plan := t.ctx.Value("current_plan").(agent.Plan)
	for _, change := range plan.Changes {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CODING", fmt.Sprintf("[%s] 수정", change.FilePath), change.Instructions, "")
		t.coder.ModifyFile(t.ctx, filepath.Join(t.repoPath, change.FilePath), change.Instructions)
	}
	return nil
}

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
