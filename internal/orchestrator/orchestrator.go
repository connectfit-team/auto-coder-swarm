package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
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

type ProjectMetadata struct {
	Type          string   `json:"type"`
	BuildCommand  string   `json:"build_command"`
	BenchCommand  string   `json:"bench_command"`
	KeyFiles      []string `json:"key_files"`
	RiskAssessment string   `json:"risk_assessment"`
}

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
}

func (o *SwarmOrchestrator) detectProjectTypeLLM(ctx context.Context, taskID uint, path string) ProjectMetadata {
	primary, _ := o.loadModels()

	var structure string
	cmd := exec.CommandContext(ctx, "tree", "-L", "3", "-F", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		cmd = exec.CommandContext(ctx, "ls", "-R", path)
		out, _ = cmd.CombinedOutput()
	}
	structure = string(out)
	if len(structure) > 4000 { structure = structure[:4000] + "..." }

	prompt := fmt.Sprintf(`레포지토리의 파일 구조를 분석하여 프로젝트의 성격과 최적의 빌드/검증 명령어를 판단해줘.
[Directory Structure]
%s

MANDATORY JSON FORMAT:
{
  "type": "Go/Flutter/Python/NodeJS/etc",
  "build_command": "상세 빌드 명령어 (예: flutter analyze, go build ./...)",
  "bench_command": "성능 측정 명령어",
  "key_files": ["주요 파일"],
  "risk_assessment": "환경적 리스크"
}
출력은 오직 JSON만 허용한다.`, structure)

	resp, _ := agent.CallLLM(ctx, primary, "Classifier", prompt)
	o.logDeepTechnical(ctx, taskID, "DETECTION_RAW", "프로젝트 분류 LLM 원본 응답", prompt, resp)
	
	var meta ProjectMetadata
	jsonStr := extractJSON(resp)
	if err := json.Unmarshal([]byte(jsonStr), &meta); err != nil {
		return ProjectMetadata{Type: "Unknown", BuildCommand: "ls"}
	}
	return meta
}

func extractJSON(raw string) string {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start != -1 && end != -1 && end > start {
		return raw[start : end+1]
	}
	return raw
}

func (o *SwarmOrchestrator) RunStatelessTask(ctx context.Context, taskID uint, req StatelessRequest, isApproved bool, repoLockFunc func(string) (bool, error)) (RunResult, error) {
	primaryLLM, v := o.loadModels()
	planner := agent.NewPlannerAgent(primaryLLM)
	coder := agent.NewCoderAgent(primaryLLM)
	reviewer := agent.NewReviewerAgent(primaryLLM)

	analysis := req.AnalysisContext
	if analysis == "" {
		sessionID := fmt.Sprintf("swarm-task-%d", taskID)
		o.logDeepTechnical(ctx, taskID, "ORACLE", "코드 인사이트 엔진(CIE) 분석 쿼리 전송", req.UserRequest, "")
		
		res, err := o.insightClient.QueryOracle(ctx, req.UserRequest, sessionID)
		if err != nil {
			o.logDeepTechnical(ctx, taskID, "ERROR", "CIE 분석 쿼리 실패", err.Error(), "")
			return RunResult{}, err
		}
		analysis = res
		o.logDeepTechnical(ctx, taskID, "INIT", "분석 컨텍스트 확보 완료", "", analysis)
	} else {
		o.logDeepTechnical(ctx, taskID, "INIT", "외부 분석 컨텍스트 주입됨", "", analysis)
	}

	wsPath, _ := o.wsMgr.CreateWorkspace()
	defer o.wsMgr.Cleanup(wsPath)

	var lastFeedback, currentBranch, targetRepo, finalDiff string
	var preBench, postBench string
	var meta ProjectMetadata

	for attempt := 1; attempt <= 3; attempt++ {
		if ctx.Err() != nil { return RunResult{}, ctx.Err() }

		o.logDeepTechnical(ctx, taskID, "PLANNING", fmt.Sprintf("코드 수정 계획 수립 중 (시도 %d/3)", attempt), analysis, "")
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
			meta = o.detectProjectTypeLLM(ctx, taskID, repoPath)
			o.logDeepTechnical(ctx, taskID, "DETECTION", fmt.Sprintf("LLM 프로젝트 판별 결과: [%s]", meta.Type), "", meta.RiskAssessment)

			if meta.BenchCommand != "" {
				cmd := exec.CommandContext(ctx, "bash", "-c", meta.BenchCommand)
				cmd.Dir = repoPath
				bOut, _ := cmd.CombinedOutput()
				preBench = string(bOut)
			}
		}

		if ctx.Err() != nil { return RunResult{}, ctx.Err() }

		for _, change := range plan.Changes {
			o.logDeepTechnical(ctx, taskID, "CODING", fmt.Sprintf("[%s] 수정 중", change.FilePath), change.Instructions, "")
			coder.ModifyFile(ctx, filepath.Join(repoPath, change.FilePath), change.Instructions)
		}

		if ctx.Err() != nil { return RunResult{}, ctx.Err() }

		o.logDeepTechnical(ctx, taskID, "BUILD", fmt.Sprintf("[%s] 검증 실행 (%s)", meta.Type, meta.BuildCommand), meta.BuildCommand, "")
		bCmd := exec.CommandContext(ctx, "bash", "-c", meta.BuildCommand)
		bCmd.Dir = repoPath
		buildOut, err := bCmd.CombinedOutput()

		if err != nil {
			if ctx.Err() != nil { return RunResult{}, ctx.Err() }
			o.logDeepTechnical(ctx, taskID, "HEALING", "빌드 실패, 자가 치유 시도", string(buildOut), "")
			lastFeedback = fmt.Sprintf("BUILD FAILED: %v\nOutput: %s", err, string(buildOut))
			exec.CommandContext(ctx, "git", "-C", repoPath, "checkout", ".").Run(); continue
		}

		if meta.BenchCommand != "" {
			cmd := exec.CommandContext(ctx, "bash", "-c", meta.BenchCommand)
			cmd.Dir = repoPath
			bOut, _ := cmd.CombinedOutput()
			postBench = string(bOut)
		}

		diffCmd := exec.CommandContext(ctx, "git", "-C", repoPath, "diff", "HEAD")
		diffOut, _ := diffCmd.CombinedOutput()
		finalDiff = string(diffOut)
		o.store.UpdateTaskProposedDiff(taskID, finalDiff)

		reviewInput := fmt.Sprintf("DIFF:\n%s\n\nPRE-BENCH:\n%s\n\nPOST-BENCH:\n%s", finalDiff, preBench, postBench)
		reviewResp, _ := reviewer.Process(ctx, reviewInput)

		if !reviewer.IsApproved(reviewResp) {
			o.logDeepTechnical(ctx, taskID, "REJECTED", "내부 감사 반려", reviewInput, reviewResp)
			lastFeedback = reviewResp; exec.CommandContext(ctx, "git", "-C", repoPath, "checkout", ".").Run(); continue
		}

		if !isApproved {
			o.logDeepTechnical(ctx, taskID, "WAIT", "기술 검증 완료. 승인 대기", "", "")
			return RunResult{RepoName: targetRepo, WaitingApproval: true}, nil
		}

		prURL, _ := o.gitMgr.PushApprovedChanges(repoPath, targetRepo, currentBranch, "feat: automated documentation")
		o.logDeepTechnical(ctx, taskID, "COMPLETED", "성공적으로 PR을 생성했습니다.", prURL, "")
		return RunResult{RepoName: targetRepo, PRURL: prURL}, nil
	}
	return RunResult{RepoName: targetRepo}, fmt.Errorf("최대 재시도 횟수(3회)를 초과했습니다. Auditor가 계속 반려 중입니다.")
}
