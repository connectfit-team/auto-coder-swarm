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

type ProjectType string

const (
	TypeGo      ProjectType = "Go"
	TypeFlutter ProjectType = "Flutter"
	TypePython  ProjectType = "Python"
	TypeNodeJS  ProjectType = "NodeJS"
	TypeUnknown ProjectType = "Unknown"
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
		summarizePrompt := fmt.Sprintf("기술 요약(Summary) 작성: %s", rawResult)
		summary, _ = agent.CallLLM(ctx, primary, "SummaryAgent", summarizePrompt)
	}
	if o.store != nil {
		o.store.AddDeepLog(taskID, stage, message, prompt, summary)
		task, _ := o.store.GetTaskByID(taskID)
		newState := fmt.Sprintf("%s\n[%s] %s: %s", task.ContextState, time.Now().Format("15:04:05"), stage, message)
		o.store.UpdateContextState(taskID, newState)
	}
	log.Printf("[T-%d] [%s] %s", taskID, stage, message)
}

func (o *SwarmOrchestrator) detectProjectType(path string) ProjectType {
	if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
		return TypeGo
	}
	if _, err := os.Stat(filepath.Join(path, "pubspec.yaml")); err == nil {
		return TypeFlutter
	}
	if _, err := os.Stat(filepath.Join(path, "requirements.txt")); err == nil {
		return TypePython
	}
	if _, err := os.Stat(filepath.Join(path, "package.json")); err == nil {
		return TypeNodeJS
	}
	return TypeUnknown
}

func (o *SwarmOrchestrator) runBuildVerification(path string, pType ProjectType) (string, error) {
	var cmd *exec.Cmd
	switch pType {
	case TypeGo:
		cmd = exec.Command("/usr/local/go/bin/go", "build", "./...")
	case TypeFlutter:
		cmd = exec.Command("/home/cnf/flutter/bin/flutter", "analyze")
	case TypeNodeJS:
		cmd = exec.Command("npm", "run", "build")
	default:
		return "Unknown project type, skipping build", nil
	}
	cmd.Dir = path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("build failed (%s): %v", pType, err)
	}
	return string(out), nil
}

func (o *SwarmOrchestrator) runBenchmark(path string, pType ProjectType) (string, error) {
	if pType == TypeGo {
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
	critic := agent.NewCriticAgent(primaryLLM)

	o.logDeepTechnical(ctx, taskID, "INIT", fmt.Sprintf("프로젝트 [%s]에 대한 수정 작업 준비 중", req.TargetRepo), "", "")

	analysis := req.AnalysisContext
	if analysis == "" {
		analysis, _ = o.insightClient.QueryOracle(ctx, req.UserRequest)
	}

	wsPath, _ := o.wsMgr.CreateWorkspace()
	defer o.wsMgr.Cleanup(wsPath)

	var lastFeedback, currentBranch, targetRepo, finalDiff string
	var preBench, postBench string
	var pType ProjectType

	for attempt := 1; attempt <= 3; attempt++ {
		o.logDeepTechnical(ctx, taskID, "PLANNING", "코드 수정 계획 수립 중", analysis, "")
		
		input := analysis
		if lastFeedback != "" { input += "\n\nFEEDBACK:\n" + lastFeedback }
		voteRes, _ := v.Vote(ctx, "Planner", planner.BuildPrompt(input))
		plan, _ := planner.ParsePlan(voteRes.Winner)

		if attempt == 1 {
			targetRepo = plan.RepoName
			if req.TargetRepo != "" { targetRepo = req.TargetRepo }
			if repoLockFunc != nil { repoLockFunc(targetRepo) }
		}

		repoPath := filepath.Join(wsPath, "repo")
		if attempt == 1 {
			currentBranch = fmt.Sprintf("swarm-fix-%d", time.Now().Unix())
			o.wsMgr.CreateWorktree(targetRepo, repoPath, currentBranch)
			
			// Intelligent Project Detection
			pType = o.detectProjectType(repoPath)
			o.logDeepTechnical(ctx, taskID, "DETECTION", fmt.Sprintf("프로젝트 타입 판별 결과: [%s]", pType), "", string(pType))
			preBench, _ = o.runBenchmark(repoPath, pType)
		}

		for _, change := range plan.Changes {
			o.logDeepTechnical(ctx, taskID, "CODING", fmt.Sprintf("[%s] 수정 중", change.FilePath), change.Instructions, "")
			coder.ModifyFile(ctx, filepath.Join(repoPath, change.FilePath), change.Instructions)
		}

		o.logDeepTechnical(ctx, taskID, "BUILD", fmt.Sprintf("[%s] 검증 빌드 실행 중", pType), "", "")
		buildOut, err := o.runBuildVerification(repoPath, pType)
		if err != nil {
			o.logDeepTechnical(ctx, taskID, "HEALING", "빌드 실패, 자동 수리 시도", err.Error(), buildOut)
			lastFeedback = fmt.Sprintf("BUILD FAILED: %v\nOutput: %s", err, buildOut)
			exec.Command("git", "-C", repoPath, "checkout", ".").Run(); continue
		}

		if preBench != "" { postBench, _ = o.runBenchmark(repoPath, pType) }
		
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
			o.logDeepTechnical(ctx, taskID, "WAIT", "기술 검증 완료. 승인 대기", "", "")
			return RunResult{RepoName: targetRepo, WaitingApproval: true}, nil
		}

		prURL, _ := o.gitMgr.PushApprovedChanges(repoPath, targetRepo, currentBranch, "feat: automated documentation")
		o.logDeepTechnical(ctx, taskID, "COMPLETED", "성공적으로 PR을 생성했습니다.", prURL, "")
		return RunResult{RepoName: targetRepo, PRURL: prURL}, nil
	}
	return RunResult{RepoName: targetRepo}, fmt.Errorf("max retries")
}
