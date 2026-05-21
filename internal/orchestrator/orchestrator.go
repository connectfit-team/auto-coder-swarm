package orchestrator

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
	"github.com/connectfit-team/auto-coder-swarm/internal/gitmgr"
	"github.com/connectfit-team/auto-coder-swarm/internal/insightclient"
	"github.com/connectfit-team/auto-coder-swarm/internal/llm"
	"github.com/connectfit-team/auto-coder-swarm/internal/storage"
	"github.com/connectfit-team/auto-coder-swarm/internal/voter"
	"github.com/connectfit-team/auto-coder-swarm/internal/workspace"
	"google.golang.org/adk/model"
)

type SwarmOrchestrator struct {
	insightClient *insightclient.Client
	wsMgr         workspace.Manager
	gitMgr        *gitmgr.GitManager
	store         *storage.Storage
}

type RunResult struct {
	RepoName        string
	PRURL           string
	WaitingApproval bool
	ChainTasks      []StatelessRequest
}

type StatelessRequest struct {
	UserRequest     string   `json:"user_request"`
	AnalysisContext string   `json:"analysis_context,omitempty"`
	TargetRepo      string   `json:"target_repo,omitempty"`
	TargetFiles     []string `json:"target_files,omitempty"`
	Constraints     []string `json:"constraints,omitempty"`
	Depth           int      `json:"depth"`
}

func NewSwarmOrchestrator(ic *insightclient.Client, ws workspace.Manager, gm *gitmgr.GitManager, s *storage.Storage) *SwarmOrchestrator {
	return &SwarmOrchestrator{
		insightClient: ic,
		wsMgr:         ws,
		gitMgr:        gm,
		store:         s,
	}
}

func (o *SwarmOrchestrator) loadModels() (model.LLM, *voter.MultiModelVoter) {
	baseURL := "http://localhost:11434"
	primaryName := o.store.GetSetting("primary_model")
	if primaryName == "" { primaryName = "gemma4:31b" }
	primary := llm.NewOllamaModel(primaryName, baseURL)

	voterNames := strings.Split(o.store.GetSetting("voter_models"), ",")
	var voterLLMs []model.LLM
	for _, name := range voterNames {
		if n := strings.TrimSpace(name); n != "" {
			voterLLMs = append(voterLLMs, llm.NewOllamaModel(n, baseURL))
		}
	}
	if len(voterLLMs) == 0 { voterLLMs = append(voterLLMs, primary) }
	return primary, voter.NewMultiModelVoter(voterLLMs...)
}

func (o *SwarmOrchestrator) logDeepTechnical(ctx context.Context, taskID uint, stage, message, prompt, rawResult string) {
	summary := ""
	if rawResult != "" {
		primary, _ := o.loadModels()
		summarizePrompt := fmt.Sprintf("다음 실행 결과를 개발자가 디버깅하기 좋게 핵심 기술 요약(Summary)으로 작성해줘.\n결과: %s", rawResult)
		summary, _ = agent.CallLLM(ctx, primary, "SummaryAgent", summarizePrompt)
	}

	if o.store != nil {
		o.store.AddDeepLog(taskID, stage, message, prompt, summary)
		
		// Update context state
		task, _ := o.store.GetTaskByID(taskID)
		timestamp := time.Now().Format("15:04:05")
		newState := fmt.Sprintf("%s\n[%s] %s: %s (Summary: %s)", task.ContextState, timestamp, stage, message, summary)
		o.store.UpdateContextState(taskID, newState)
	}
	log.Printf("[T-%d] [%s] %s", taskID, stage, message)
}

func (o *SwarmOrchestrator) runBuildVerification(path string) (string, error) {
	var cmd *exec.Cmd
	if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
		cmd = exec.Command("/usr/local/go/bin/go", "build", "./...")
	} else if _, err := os.Stat(filepath.Join(path, "pubspec.yaml")); err == nil {
		cmd = exec.Command("/home/cnf/flutter/bin/flutter", "analyze")
	}
	if cmd == nil { return "", nil }
	cmd.Dir = path
	out, err := cmd.CombinedOutput()
	if err != nil { return string(out), fmt.Errorf("build failed: %v", err) }
	return string(out), nil
}

func (o *SwarmOrchestrator) runBenchmark(path string) (string, error) {
	if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
		cmd := exec.Command("/usr/local/go/bin/go", "test", "-bench=.", "-benchmem", "./...")
		cmd.Dir = path
		out, err := cmd.CombinedOutput()
		if err != nil { return "", err }
		return string(out), nil
	}
	return "", nil
}

func (o *SwarmOrchestrator) RunStatelessTask(ctx context.Context, taskID uint, req StatelessRequest, isApproved bool, repoLockFunc func(string) (bool, error)) (RunResult, error) {
	primaryLLM, v := o.loadModels()
	planner := agent.NewPlannerAgent(primaryLLM)
	coder := agent.NewCoderAgent(primaryLLM)
	reviewer := agent.NewReviewerAgent(primaryLLM)
	riskAssessor := agent.NewRiskAssessorAgent(primaryLLM)
	critic := agent.NewCriticAgent(primaryLLM)
	summarizer := agent.NewSummarizerAgent(primaryLLM)

	o.logDeepTechnical(ctx, taskID, "INIT", fmt.Sprintf("프로젝트 [%s]에 대한 수정 작업 준비 중", req.TargetRepo), "", "Environment initialized")

	analysis := req.AnalysisContext
	if analysis == "" {
		prompt := req.UserRequest
		o.logDeepTechnical(ctx, taskID, "ORACLE", fmt.Sprintf("분석 요청: '%s'", prompt), prompt, "Requesting Oracle analysis")
		var err error
		analysis, err = o.insightClient.QueryOracle(ctx, prompt)
		if err != nil {
			o.logDeepTechnical(ctx, taskID, "ERROR", "분석 엔진 응답 실패", prompt, err.Error())
			return RunResult{}, err
		}
	}

	wsPath, err := o.wsMgr.CreateWorkspace()
	if err != nil { return RunResult{}, err }
	defer o.wsMgr.Cleanup(wsPath)

	var lastFeedback, currentBranch, targetRepo, finalDiff string
	var preBench, postBench string
	maxRetries := 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		o.logDeepTechnical(ctx, taskID, "PLANNING", fmt.Sprintf("[%d회차] 계획 수립 중", attempt), analysis, "Starting plan extraction")
		
		input := analysis
		if lastFeedback != "" { input = fmt.Sprintf("ANALYSIS:\n%s\n\nFEEDBACK:\n%s", analysis, lastFeedback) }

		voteRes, _ := v.Vote(ctx, "Planner", planner.BuildPrompt(input))
		planRaw := voteRes.Winner

		plan, err := planner.ParsePlan(planRaw)
		if err == nil {
			var files []string
			for _, c := range plan.Changes { files = append(files, c.FilePath) }
			o.logDeepTechnical(ctx, taskID, "PLAN_READY", fmt.Sprintf("대상 파일 확정: %s", strings.Join(files, ", ")), planner.BuildPrompt(input), planRaw)
		}

		criticism, _ := critic.Process(ctx, planRaw)
		if !strings.Contains(strings.ToUpper(criticism), "PERFECT") {
			o.logDeepTechnical(ctx, taskID, "REFINE", "비판 내용을 반영하여 계획 보강 중", planRaw, criticism)
			planRaw, _ = planner.Refine(ctx, analysis, planRaw, criticism)
			plan, _ = planner.ParsePlan(planRaw)
		}

		if attempt == 1 {
			targetRepo = plan.RepoName
			if req.TargetRepo != "" { targetRepo = req.TargetRepo }
			if repoLockFunc != nil {
				locked, err := repoLockFunc(targetRepo)
				if err != nil || !locked { return RunResult{RepoName: targetRepo}, fmt.Errorf("repo locked") }
			}
		}

		repoPath := filepath.Join(wsPath, "repo")
		if attempt == 1 {
			currentBranch = fmt.Sprintf("swarm-fix-%d", time.Now().Unix())
			o.wsMgr.CreateWorktree(targetRepo, repoPath, currentBranch)
			preBench, _ = o.runBenchmark(repoPath)
		}

		for _, change := range plan.Changes {
			o.logDeepTechnical(ctx, taskID, "CODING", fmt.Sprintf("[%s] 파일 수정 중", change.FilePath), change.Instructions, "Writing code and comments")
			coder.ModifyFile(ctx, filepath.Join(repoPath, change.FilePath), change.Instructions)
		}

		o.logDeepTechnical(ctx, taskID, "BUILD", fmt.Sprintf("[%s] 검증 중", targetRepo), "go build ./...", "Checking build integrity")
		buildOut, err := o.runBuildVerification(repoPath)
		if err != nil {
			o.logDeepTechnical(ctx, taskID, "HEALING", "빌드 실패, 자가 치유 시도 중", err.Error(), buildOut)
			lastFeedback = fmt.Sprintf("BUILD FAILED: %v\nOutput: %s", err, buildOut)
			exec.Command("git", "-C", repoPath, "checkout", ".").Run(); continue
		}

		diffCmd := exec.Command("git", "-C", repoPath, "diff", "HEAD")
		diffOut, _ := diffCmd.CombinedOutput()
		finalDiff = string(diffOut)
		o.store.UpdateTaskProposedDiff(taskID, finalDiff)

		reviewInput := fmt.Sprintf("DIFF:\n%s\n\nPRE-BENCH:\n%s\n\nPOST-BENCH:\n%s", finalDiff, preBench, postBench)
		reviewResp, _ := reviewer.Process(ctx, reviewInput)

		if !reviewer.IsApproved(reviewResp) {
			o.logDeepTechnical(ctx, taskID, "REJECTED", "내부 감사 반려", reviewInput, reviewResp)
			lastFeedback = reviewResp; exec.Command("git", "-C", repoPath, "checkout", ".").Run(); continue
		}

		if !isApproved {
			o.logDeepTechnical(ctx, taskID, "WAIT", "기술 검증 완료. 승인 대기", "", "Waiting for human approval")
			return RunResult{RepoName: targetRepo, WaitingApproval: true}, nil
		}

		prURL, _ := o.gitMgr.PushApprovedChanges(repoPath, targetRepo, currentBranch, "feat: automated documentation")
		o.logDeepTechnical(ctx, taskID, "COMPLETED", "PR 생성 완료", prURL, "Task successfully finished")
		return RunResult{RepoName: targetRepo, PRURL: prURL}, nil
	}
	return RunResult{RepoName: targetRepo}, fmt.Errorf("max retries")
}
