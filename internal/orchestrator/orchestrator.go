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
		name = strings.TrimSpace(name)
		if name != "" {
			voterLLMs = append(voterLLMs, llm.NewOllamaModel(name, baseURL))
		}
	}
	
	if len(voterLLMs) == 0 {
		voterLLMs = append(voterLLMs, primary)
	}

	return primary, voter.NewMultiModelVoter(voterLLMs...)
}

func (o *SwarmOrchestrator) reportStatus(logID, stage, message string) {
	timestamp := time.Now().Format("15:04:05")
	log.Printf("[%s] [%s] [%s] %s", logID, timestamp, stage, message)
}

func (o *SwarmOrchestrator) recordStage(taskID uint, stage, message string) {
	if o.store != nil {
		o.store.AddLog(taskID, stage, message)
	}
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
	if err != nil {
		return string(out), fmt.Errorf("build failed: %v", err)
	}
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

func (o *SwarmOrchestrator) trySelfHealing(logID string, path, errMsg string) bool {
	if strings.Contains(errMsg, "module") || strings.Contains(errMsg, "go.sum") {
		cmd := exec.Command("/usr/local/go/bin/go", "mod", "tidy")
		cmd.Dir = path
		return cmd.Run() == nil
	}
	if strings.Contains(errMsg, "pub get") {
		cmd := exec.Command("/home/cnf/flutter/bin/flutter", "pub", "get")
		cmd.Dir = path
		return cmd.Run() == nil
	}
	return false
}

func (o *SwarmOrchestrator) detectChainTasks(ctx context.Context, llm model.LLM, targetRepo, diff string, depth int) []StatelessRequest {
	if depth >= 2 { return nil }
	// Placeholder for MSA chain detection logic
	return nil
}

func (o *SwarmOrchestrator) RunStatelessTask(ctx context.Context, taskID uint, req StatelessRequest, isApproved bool, repoLockFunc func(string) (bool, error)) (RunResult, error) {
	primaryLLM, v := o.loadModels()
	
	planner := agent.NewPlannerAgent(primaryLLM)
	coder := agent.NewCoderAgent(primaryLLM)
	reviewer := agent.NewReviewerAgent(primaryLLM)
	riskAssessor := agent.NewRiskAssessorAgent(primaryLLM)
	critic := agent.NewCriticAgent(primaryLLM)
	summarizer := agent.NewSummarizerAgent(primaryLLM)

	logID := fmt.Sprintf("T-%d", taskID)
	o.reportStatus(logID, "INIT", fmt.Sprintf("Task start (Depth: %d): %s", req.Depth, req.UserRequest))
	o.recordStage(taskID, "INIT", "분석 및 작업 준비 시작")

	analysis := req.AnalysisContext
	if analysis == "" {
		o.recordStage(taskID, "ORACLE", "Oracle 분석 요청 중...")
		var err error
		analysis, err = o.insightClient.QueryOracle(ctx, req.UserRequest)
		if err != nil {
			o.recordStage(taskID, "ERROR", "Oracle 분석 실패: "+err.Error())
			return RunResult{}, err
		}
	}

	if len(analysis) > 3000 {
		o.recordStage(taskID, "SUMMARY", "분석 요약 중...")
		compressed, _ := summarizer.Process(ctx, analysis)
		if compressed != "" { analysis = compressed }
	}

	wsPath, err := o.wsMgr.CreateWorkspace()
	if err != nil { return RunResult{}, err }
	defer o.wsMgr.Cleanup(wsPath)

	var lastFeedback, currentBranch, targetRepo, finalDiff string
	var preBench, postBench string
	maxRetries := 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		o.recordStage(taskID, "PLANNING", fmt.Sprintf("계획 수립 (시도 %d)...", attempt))
		
		input := analysis
		if lastFeedback != "" {
			input = fmt.Sprintf("ANALYSIS:\n%s\n\nPREVIOUS FEEDBACK:\n%s", analysis, lastFeedback)
		}

		o.recordStage(taskID, "VOTING", "투표 진행 중...")
		voteRes, _ := v.Vote(ctx, "Planner", planner.BuildPrompt(input))
		planRaw := voteRes.Winner

		criticism, _ := critic.Process(ctx, planRaw)
		if !strings.Contains(strings.ToUpper(criticism), "PERFECT") {
			o.recordStage(taskID, "REFINE", "계획 수정 중...")
			planRaw, _ = planner.Refine(ctx, analysis, planRaw, criticism)
		}

		plan, err := planner.ParsePlan(planRaw)
		if err != nil { return RunResult{}, err }

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
			o.recordStage(taskID, "CODING", "수정 중: "+change.FilePath)
			coder.ModifyFile(ctx, filepath.Join(repoPath, change.FilePath), change.Instructions)
			ext := filepath.Ext(change.FilePath)
			if ext == ".go" || ext == ".dart" { coder.GenerateTestFile(ctx, filepath.Join(repoPath, change.FilePath)) }
		}

		o.recordStage(taskID, "BUILD", "검증 중...")
		buildOut, err := o.runBuildVerification(repoPath)
		if err != nil {
			o.recordStage(taskID, "HEALING", "치유 시도...")
			// Simplification for recovery logic
			lastFeedback = fmt.Sprintf("BUILD FAILED: %v\nOutput: %s", err, buildOut)
			exec.Command("git", "-C", repoPath, "checkout", ".").Run(); continue
		}

		if preBench != "" { postBench, _ = o.runBenchmark(repoPath) }
		
		diffCmd := exec.Command("git", "-C", repoPath, "diff", "HEAD")
		diffOut, _ := diffCmd.CombinedOutput()
		finalDiff = string(diffOut)
		o.store.UpdateTaskProposedDiff(taskID, finalDiff)

		voteRisk, _ := v.Vote(ctx, "RiskAssessor", riskAssessor.BuildPrompt(finalDiff))
		reviewInput := fmt.Sprintf("DIFF:\n%s\n\nPRE-BENCH:\n%s\n\nPOST-BENCH:\n%s", finalDiff, preBench, postBench)
		reviewResp, _ := reviewer.Process(ctx, reviewInput)

		if !reviewer.IsApproved(reviewResp) || !riskAssessor.IsSafe(voteRisk.Winner) {
			lastFeedback = reviewResp + "\n" + voteRisk.Winner
			exec.Command("git", "-C", repoPath, "checkout", ".").Run(); continue
		}

		if !isApproved {
			o.recordStage(taskID, "WAIT", "승인 대기")
			return RunResult{RepoName: targetRepo, WaitingApproval: true}, nil
		}

		prURL, _ := o.gitMgr.PushApprovedChanges(repoPath, targetRepo, currentBranch, "feat: automated modification")
		
		syncPrompt := fmt.Sprintf("Summarize: %s", finalDiff)
		summary, _ := agent.CallLLM(ctx, primaryLLM, "Architect", syncPrompt)
		o.insightClient.UpdateKnowledge(ctx, targetRepo, summary, "automated-fix")

		chainTasks := o.detectChainTasks(ctx, primaryLLM, targetRepo, finalDiff, req.Depth)
		o.recordStage(taskID, "COMPLETED", "완료")
		return RunResult{RepoName: targetRepo, PRURL: prURL, ChainTasks: chainTasks}, nil
	}
	return RunResult{RepoName: targetRepo}, fmt.Errorf("max retries")
}
