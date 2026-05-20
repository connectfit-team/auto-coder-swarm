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
	"encoding/json"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
	"github.com/connectfit-team/auto-coder-swarm/internal/gitmgr"
	"github.com/connectfit-team/auto-coder-swarm/internal/insightclient"
	"github.com/connectfit-team/auto-coder-swarm/internal/workspace"
	"github.com/connectfit-team/auto-coder-swarm/internal/storage"
	"github.com/connectfit-team/auto-coder-swarm/internal/voter"
	"google.golang.org/adk/model"
)

type SwarmOrchestrator struct {
	insightClient *insightclient.Client
	wsMgr         workspace.Manager
	gitMgr        *gitmgr.GitManager
	planner       *agent.PlannerAgent
	coder         *agent.CoderAgent
	reviewer      *agent.ReviewerAgent
	riskAssessor  *agent.RiskAssessorAgent
	critic        *agent.CriticAgent
	summarizer    *agent.SummarizerAgent
	llm           model.LLM
	store         *storage.Storage
	voter         *voter.MultiModelVoter
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

func NewSwarmOrchestrator(ic *insightclient.Client, ws workspace.Manager, gm *gitmgr.GitManager, llm model.LLM, s *storage.Storage, v *voter.MultiModelVoter) *SwarmOrchestrator {
	return &SwarmOrchestrator{
		insightClient: ic,
		wsMgr:         ws,
		gitMgr:        gm,
		planner:       agent.NewPlannerAgent(llm),
		coder:         agent.NewCoderAgent(llm),
		reviewer:      agent.NewReviewerAgent(llm),
		riskAssessor:  agent.NewRiskAssessorAgent(llm),
		critic:        agent.NewCriticAgent(llm),
		summarizer:    agent.NewSummarizerAgent(llm),
		llm:           llm,
		store:         s,
		voter:         v,
	}
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

func (o *SwarmOrchestrator) detectChainTasks(ctx context.Context, logID string, repoName string, diff string, currentDepth int) []StatelessRequest {
	if currentDepth >= 2 { return nil }
	prompt := fmt.Sprintf("You are the MSA Dependency Architect.\n"+
		"Based on the following changes in [%s], identify other repositories that need to be updated.\n"+
		"Return JSON array: [{\"repo_name\": \"...\", \"user_request\": \"...\"}]\n\n"+
		"[Changes]\n%s", repoName, diff)
	resp, err := agent.CallLLM(ctx, o.llm, "Architect", prompt)
	if err != nil { return nil }
	var findings []struct {
		RepoName    string `json:"repo_name"`
		UserRequest string `json:"user_request"`
	}
	start := strings.Index(resp, "["); end := strings.LastIndex(resp, "]")
	if start == -1 || end == -1 { return nil }
	if err := json.Unmarshal([]byte(resp[start:end+1]), &findings); err != nil { return nil }
	var tasks []StatelessRequest
	for _, f := range findings {
		tasks = append(tasks, StatelessRequest{UserRequest: f.UserRequest, TargetRepo: f.RepoName, Depth: currentDepth + 1})
	}
	return tasks
}

func (o *SwarmOrchestrator) RunStatelessTask(ctx context.Context, taskID uint, req StatelessRequest, isApproved bool, repoLockFunc func(string) (bool, error)) (RunResult, error) {
	logID := fmt.Sprintf("T-%d", taskID)
	o.reportStatus(logID, "INIT", fmt.Sprintf("Task start (Depth: %d): %s", req.Depth, req.UserRequest))
	o.recordStage(taskID, "INIT", "분석 및 작업 준비 시작")

	analysis := req.AnalysisContext
	if analysis == "" {
		o.recordStage(taskID, "ORACLE", "Oracle(엔진)에 코드 맥락 분석 요청 중...")
		var err error
		analysis, err = o.insightClient.QueryOracle(ctx, req.UserRequest)
		if err != nil {
			o.recordStage(taskID, "ERROR", "Oracle 분석 실패: "+err.Error())
			return RunResult{}, err
		}
	}

	if len(analysis) > 3000 {
		o.recordStage(taskID, "SUMMARY", "분석 내용 요약 및 압축 중...")
		compressed, _ := o.summarizer.Process(ctx, analysis)
		if compressed != "" { analysis = compressed }
	}

	wsPath, err := o.wsMgr.CreateWorkspace()
	if err != nil { return RunResult{}, err }
	defer o.wsMgr.Cleanup(wsPath)

	var lastFeedback, currentBranch, targetRepo, finalDiff string
	var preBench, postBench string
	maxRetries := 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		o.reportStatus(logID, "PLANNING", fmt.Sprintf("Attempt %d...", attempt))
		o.recordStage(taskID, "PLANNING", fmt.Sprintf("수정 계획 수립 중 (시도 %d)...", attempt))
		
		input := analysis
		if lastFeedback != "" {
			input = fmt.Sprintf("ANALYSIS:\n%s\n\nPREVIOUS REVIEW FEEDBACK:\n%s", analysis, lastFeedback)
		}

		// STEP 26: Multi-Model Voting for Planning
		var planRaw string
		if o.voter != nil {
			o.recordStage(taskID, "VOTING", "다중 모델 투표 기반 최적 계획 선정 중 (Step 26)...")
			prompt := o.planner.BuildPrompt(input)
			voteRes, _ := o.voter.Vote(ctx, "Planner", prompt)
			planRaw = voteRes.Winner
		} else {
			planRaw, _ = o.planner.Process(ctx, input)
		}

		// STEP 33: Agentic Dialogue (Critic Loop)
		o.recordStage(taskID, "DIALOGUE", "에이전트 간 비판 및 보완 토론 진행 중 (Step 33)...")
		criticism, _ := o.critic.Process(ctx, planRaw)
		if strings.Contains(strings.ToUpper(criticism), "PERFECT") {
			o.recordStage(taskID, "CONSENSUS", "계획이 완벽함으로 합의됨.")
		} else {
			o.recordStage(taskID, "REFINE", "비판 의견 수용 및 계획 보강 중...")
			refinedPlan, _ := o.planner.Refine(ctx, analysis, planRaw, criticism)
			planRaw = refinedPlan
		}
		
		plan, err := o.planner.ParsePlan(planRaw)
		if err != nil { return RunResult{}, err }

		if attempt == 1 {
			targetRepo = plan.RepoName
			if req.TargetRepo != "" { targetRepo = req.TargetRepo }
			o.recordStage(taskID, "LOCK", "저장소 점유 확인: "+targetRepo)
			if repoLockFunc != nil {
				locked, err := repoLockFunc(targetRepo)
				if err != nil { return RunResult{RepoName: targetRepo}, err }
				if !locked { return RunResult{RepoName: targetRepo}, fmt.Errorf("repo %s locked", targetRepo) }
			}
		}

		repoPath := filepath.Join(wsPath, "repo")
		if attempt == 1 {
			// STEP 32: Instant Sandboxing via Git Worktree
			o.recordStage(taskID, "SANDBOX", "Git Worktree 기반 초고속 작업 공간 생성 중 (Step 32)...")
			currentBranch = fmt.Sprintf("swarm-fix-%d", time.Now().Unix())
			if err := o.wsMgr.CreateWorktree(targetRepo, repoPath, currentBranch); err != nil {
				o.recordStage(taskID, "ERROR", "Worktree 생성 실패: "+err.Error())
				return RunResult{RepoName: targetRepo}, err
			}
			preBench, _ = o.runBenchmark(repoPath)
		}

		for _, change := range plan.Changes {
			o.recordStage(taskID, "CODING", "파일 수정 적용: "+change.FilePath)
			o.coder.ModifyFile(ctx, filepath.Join(repoPath, change.FilePath), change.Instructions)
			ext := filepath.Ext(change.FilePath)
			if ext == ".go" || ext == ".dart" { o.coder.GenerateTestFile(ctx, filepath.Join(repoPath, change.FilePath)) }
		}

		isDocOnly := true
		for _, c := range plan.Changes {
			if !strings.HasSuffix(c.FilePath, ".md") { isDocOnly = false; break }
		}
		if !isDocOnly {
			o.recordStage(taskID, "BUILD", "빌드 및 정적 분석 수행 중 (Flutter/Go)...")
			buildOut, err := o.runBuildVerification(repoPath)
			if err != nil {
				// STEP 31: Agentic Self-Healing Pro
				o.recordStage(taskID, "HEALING", "빌드 실패 감지. 에러 로그 분석 및 자가 치유 시도 중 (Step 31)...")
				
				if len(plan.Changes) > 0 {
					repairFile := filepath.Join(repoPath, plan.Changes[0].FilePath)
					repairRes, repairErr := o.coder.RepairFile(ctx, repairFile, plan.Changes[0].Instructions, buildOut)
					if repairErr == nil {
						o.recordStage(taskID, "HEALING_FIX", "치유 시도 완료: "+repairRes+". 재검증 중...")
						buildOut, err = o.runBuildVerification(repoPath)
						if err == nil { goto build_passed }
					}
				}

				if o.trySelfHealing(logID, repoPath, err.Error()) {
					if _, err = o.runBuildVerification(repoPath); err == nil { goto build_passed }
				}
				o.recordStage(taskID, "RETRY", "자가 치유 실패, 계획 재검토 중...")
				lastFeedback = fmt.Sprintf("BUILD FAILED: %v\nOutput: %s", err, buildOut)
				exec.Command("git", "-C", repoPath, "checkout", ".").Run()
				continue
			}
		}

	build_passed:
		if !isDocOnly && preBench != "" { postBench, _ = o.runBenchmark(repoPath) }

		diffCmd := exec.Command("git", "-C", repoPath, "diff", "HEAD")
		diffOut, _ := diffCmd.CombinedOutput()
		finalDiff = string(diffOut)
		o.store.UpdateTaskProposedDiff(taskID, finalDiff)

		o.reportStatus(logID, "AUDIT", "Parallel Audits...")
		o.recordStage(taskID, "AUDIT", "코드 리뷰 및 리스크 평가 진행 중...")
		
		var riskResp string
		if o.voter != nil {
			riskPrompt := o.riskAssessor.BuildPrompt(finalDiff)
			voteRes, _ := o.voter.Vote(ctx, "RiskAssessor", riskPrompt)
			riskResp = voteRes.Winner
		} else {
			riskResp, _ = o.riskAssessor.Process(ctx, finalDiff)
		}

		var reviewResp string
		var reviewErr error
		reviewInput := fmt.Sprintf("DIFF:\n%s\n\nPRE-BENCHMARK:\n%s\n\nPOST-BENCHMARK:\n%s", finalDiff, preBench, postBench)
		reviewResp, reviewErr = o.reviewer.Process(ctx, reviewInput) 

		if reviewErr != nil { return RunResult{RepoName: targetRepo}, fmt.Errorf("audit failed") }

		if !o.reviewer.IsApproved(reviewResp) {
			o.recordStage(taskID, "REJECTED", "코드 리뷰 반려: "+reviewResp)
			lastFeedback = reviewResp; exec.Command("git", "-C", repoPath, "checkout", ".").Run(); continue
		}
		if !o.riskAssessor.IsSafe(riskResp) {
			o.recordStage(taskID, "RISK", "보안/성능 리스크 감지: "+riskResp)
			lastFeedback = riskResp; exec.Command("git", "-C", repoPath, "checkout", ".").Run(); continue
		}

		if !isApproved {
			o.reportStatus(logID, "WAIT", "Approval required.")
			o.recordStage(taskID, "WAIT", "검증 완료. 대시보드에서 코드 변경 사항(Diff)을 검토해 주세요.")
			return RunResult{RepoName: targetRepo, WaitingApproval: true}, nil
		}

		task, _ := o.store.GetTaskByID(taskID)
		if task.HumanFeedback != "" {
			o.recordStage(taskID, "FEEDBACK", "사용자 피드백 반영 중: "+task.HumanFeedback)
			lastFeedback = "USER FEEDBACK:\n" + task.HumanFeedback
			o.store.UpdateHumanFeedback(taskID, "")
			exec.Command("git", "-C", repoPath, "checkout", ".").Run()
			continue
		}

		o.reportStatus(logID, "SUCCESS", "Creating PR...")
		o.reportStatus(logID, "SUCCESS", "GitHub Pull Request 생성 중...")
		prURL, err := o.gitMgr.PushApprovedChanges(repoPath, targetRepo, currentBranch, "feat: automated modification")
		if err != nil { 
			o.recordStage(taskID, "ERROR", "PR 생성 실패: "+err.Error())
			return RunResult{RepoName: targetRepo}, err 
		}

		o.recordStage(taskID, "KNOWLEDGE", "Oracle 지식 동기화 중...")
		o.reportStatus(logID, "KNOWLEDGE", "Syncing to Oracle...")
		syncPrompt := fmt.Sprintf("Summarize changes in [%s] in 2-3 sentences.\n\n[Changes]\n%s", targetRepo, finalDiff)
		summary, _ := agent.CallLLM(ctx, o.llm, "Architect", syncPrompt)
		o.insightClient.UpdateKnowledge(ctx, targetRepo, summary, "automated-fix")

		chainTasks := o.detectChainTasks(ctx, logID, targetRepo, finalDiff, req.Depth)
		o.recordStage(taskID, "COMPLETED", "작업 완료")
		return RunResult{RepoName: targetRepo, PRURL: prURL, ChainTasks: chainTasks}, nil
	}
	o.recordStage(taskID, "FAILED", "최대 재시도 횟수 초과로 실패")
	return RunResult{RepoName: targetRepo}, fmt.Errorf("max retries")
}
