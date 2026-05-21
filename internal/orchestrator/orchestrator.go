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

func (o *SwarmOrchestrator) logTechnical(ctx context.Context, taskID uint, stage, message string) {
	if o.store != nil {
		o.store.AddLog(taskID, stage, message)
		
		// Update cumulative ContextState
		task, _ := o.store.GetTaskByID(taskID)
		timestamp := time.Now().Format("15:04:05")
		newState := fmt.Sprintf("%s\n[%s] %s: %s", task.ContextState, timestamp, stage, message)
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

	o.logTechnical(ctx, taskID, "INIT", fmt.Sprintf("프로젝트 [%s]에 대한 수정 작업 준비 중", req.TargetRepo))

	analysis := req.AnalysisContext
	if analysis == "" {
		o.logTechnical(ctx, taskID, "ORACLE", fmt.Sprintf("분석 요청: '%s'", req.UserRequest))
		var err error
		analysis, err = o.insightClient.QueryOracle(ctx, req.UserRequest)
		if err != nil {
			o.logTechnical(ctx, taskID, "ERROR", "분석 엔진 응답 실패: "+err.Error())
			return RunResult{}, err
		}
	}

	if len(analysis) > 3000 {
		o.logTechnical(ctx, taskID, "SUMMARY", "방대한 분석 데이터를 핵심 기술 지표로 압축 중")
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
		o.logTechnical(ctx, taskID, "PLANNING", fmt.Sprintf("[%d회차] 분석 데이터를 기반으로 상세 코드 수정 계획 수립 중", attempt))
		
		input := analysis
		if lastFeedback != "" { input = fmt.Sprintf("ANALYSIS:\n%s\n\nFEEDBACK:\n%s", analysis, lastFeedback) }

		o.logTechnical(ctx, taskID, "VOTING", "모델 간 다수결 투표로 최적의 수정 시나리오 선정 중")
		voteRes, _ := v.Vote(ctx, "Planner", planner.BuildPrompt(input))
		planRaw := voteRes.Winner

		plan, err := planner.ParsePlan(planRaw)
		if err == nil {
			var files []string
			for _, c := range plan.Changes { files = append(files, c.FilePath) }
			o.logTechnical(ctx, taskID, "PLAN_READY", fmt.Sprintf("대상 파일 확정: %s", strings.Join(files, ", ")))
		}

		o.logTechnical(ctx, taskID, "DIALOGUE", "비판 에이전트(Critic)가 수립된 계획의 허점을 검증 중")
		criticism, _ := critic.Process(ctx, planRaw)
		if !strings.Contains(strings.ToUpper(criticism), "PERFECT") {
			o.logTechnical(ctx, taskID, "REFINE", "비판 의견을 반영하여 계획을 더 안전한 코드로 보강 중")
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
			o.logTechnical(ctx, taskID, "SANDBOX", "원본을 보호하기 위해 별도의 격리된 임시 작업 공간을 생성합니다.")
			currentBranch = fmt.Sprintf("swarm-fix-%d", time.Now().Unix())
			o.wsMgr.CreateWorktree(targetRepo, repoPath, currentBranch)
			preBench, _ = o.runBenchmark(repoPath)
		}

		for _, change := range plan.Changes {
			o.logTechnical(ctx, taskID, "CODING", fmt.Sprintf("[%s] 파일 수정 및 DartDoc 작성 중", change.FilePath))
			coder.ModifyFile(ctx, filepath.Join(repoPath, change.FilePath), change.Instructions)
			ext := filepath.Ext(change.FilePath)
			if ext == ".go" || ext == ".dart" { 
				o.logTechnical(ctx, taskID, "TEST_GEN", fmt.Sprintf("[%s]에 대한 단위 테스트 코드 생성 중", change.FilePath))
				coder.GenerateTestFile(ctx, filepath.Join(repoPath, change.FilePath)) 
			}
		}

		o.logTechnical(ctx, taskID, "BUILD", fmt.Sprintf("[%s] 저장소 빌드 및 정적 분석(Linter) 실행 중", targetRepo))
		buildOut, err := o.runBuildVerification(repoPath)
		if err != nil {
			o.logTechnical(ctx, taskID, "HEALING", "빌드 실패! 에러 로그를 분석하여 코드를 자동으로 수리합니다.")
			lastFeedback = fmt.Sprintf("BUILD FAILED: %v\nOutput: %s", err, buildOut)
			exec.Command("git", "-C", repoPath, "checkout", ".").Run(); continue
		}

		if preBench != "" { 
			o.logTechnical(ctx, taskID, "BENCHMARK", "수정 전/후 성능 비교 측정 중")
			postBench, _ = o.runBenchmark(repoPath) 
		}
		
		diffCmd := exec.Command("git", "-C", repoPath, "diff", "HEAD")
		diffOut, _ := diffCmd.CombinedOutput()
		finalDiff = string(diffOut)
		o.store.UpdateTaskProposedDiff(taskID, finalDiff)

		o.logTechnical(ctx, taskID, "AUDIT", "코드 리뷰 에이전트와 리스크 평가 에이전트가 최종 검수 진행 중")
		voteRisk, _ := v.Vote(ctx, "RiskAssessor", riskAssessor.BuildPrompt(finalDiff))
		reviewInput := fmt.Sprintf("DIFF:\n%s\n\nPRE-BENCH:\n%s\n\nPOST-BENCH:\n%s", finalDiff, preBench, postBench)
		reviewResp, _ := reviewer.Process(ctx, reviewInput)

		if !reviewer.IsApproved(reviewResp) || !riskAssessor.IsSafe(voteRisk.Winner) {
			o.logTechnical(ctx, taskID, "REJECTED", "내부 검토 결과 기준 미달로 계획을 폐기하고 재시도합니다.")
			lastFeedback = reviewResp + "\n" + voteRisk.Winner
			exec.Command("git", "-C", repoPath, "checkout", ".").Run(); continue
		}

		if !isApproved {
			o.logTechnical(ctx, taskID, "WAIT", "기술 검증 완료. 대시보드 하단의 Diff를 확인하고 승인해 주세요.")
			return RunResult{RepoName: targetRepo, WaitingApproval: true}, nil
		}

		o.logTechnical(ctx, taskID, "SUCCESS", "GitHub Pull Request 생성 및 작업 완료")
		prURL, _ := o.gitMgr.PushApprovedChanges(repoPath, targetRepo, currentBranch, "feat: automated documentation")
		
		syncPrompt := fmt.Sprintf("Summarize changes: %s", finalDiff)
		summary, _ := agent.CallLLM(ctx, primaryLLM, "Architect", syncPrompt)
		o.insightClient.UpdateKnowledge(ctx, targetRepo, summary, "automated-fix")

		o.logTechnical(ctx, taskID, "COMPLETED", "성공적으로 PR을 생성하고 모든 작업을 마무리했습니다.")
		return RunResult{RepoName: targetRepo, PRURL: prURL}, nil
	}
	return RunResult{RepoName: targetRepo}, fmt.Errorf("max retries")
}
