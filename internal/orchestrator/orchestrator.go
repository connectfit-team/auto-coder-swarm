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
	
	// [Multi-Agent Architecture] Initializing specialized agents
	planner := agent.NewPlannerAgent(primaryLLM)
	coder := agent.NewCoderAgent(primaryLLM)
	reviewer := agent.NewReviewerAgent(primaryLLM)
	critic := agent.NewCriticAgent(primaryLLM) // Added Critic for risk & architectural oversight

	analysis := req.AnalysisContext
	if analysis == "" {
		sessionID := fmt.Sprintf("swarm-task-%d", taskID)
		
		// [Intelligence-First] Swarm generates a technical analysis prompt for CIE
		oraclePrompt := fmt.Sprintf("[User Request]\n%s\n\n위 요청을 수행하기 위해 레포지토리를 정밀 분석해줘.\n1. 수정이 필요한 대상 파일들의 정확한 경로 목록\n2. 각 파일별 수정/추가해야 할 코드에 대한 상세 가이드 (함수명, 로직 설명 등)\n3. 해당 작업 시 주의해야 할 의존성이나 사이드 이펙트\n분석 결과는 나(Auto-Coder Swarm)의 코딩 에이전트가 즉시 작업에 착수할 수 있도록 구체적인 '기술 설계서' 형태로 제공해라.", req.UserRequest)

		o.logDeepTechnical(ctx, taskID, "ORACLE", "코드 인사이트 엔진(CIE) 지능형 분석 요청 전송", oraclePrompt, "")
		
		res, err := o.insightClient.QueryOracle(ctx, oraclePrompt, sessionID)
		if err != nil {
			o.logDeepTechnical(ctx, taskID, "ERROR", "CIE 분석 쿼리 실패", err.Error(), "")
			return RunResult{}, err
		}
		analysis = res
		o.logDeepTechnical(ctx, taskID, "INIT", "분석 컨텍스트 확보 완료 (기술 설계서 수신)", "", analysis)
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

		// Step 1: Planning with Voter Consensus
		o.logDeepTechnical(ctx, taskID, "PLANNING", fmt.Sprintf("다중 에이전트 합의 기반 계획 수립 (시도 %d/3)", attempt), analysis, "")
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
			
			// Step 2: Intelligent Project Detection & Risk Assessment
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

		// Step 3: Coding (Execution)
		for _, change := range plan.Changes {
			o.logDeepTechnical(ctx, taskID, "CODING", fmt.Sprintf("[%s] 수정 중", change.FilePath), change.Instructions, "")
			coder.ModifyFile(ctx, filepath.Join(repoPath, change.FilePath), change.Instructions)
		}

		if ctx.Err() != nil { return RunResult{}, ctx.Err() }

		// Step 4: Verification (Build/Analyze)
		o.logDeepTechnical(ctx, taskID, "BUILD", fmt.Sprintf("[%s] 검증 실행 (%s)", meta.Type, meta.BuildCommand), meta.BuildCommand, "")
		bCmd := exec.CommandContext(ctx, "bash", "-c", meta.BuildCommand)
		bCmd.Dir = repoPath
		buildOut, err := bCmd.CombinedOutput()

		if err != nil {
			if ctx.Err() != nil { return RunResult{}, ctx.Err() }
			o.logDeepTechnical(ctx, taskID, "HEALING", "빌드 실패, 자가 치유 및 계획 재수립", string(buildOut), "")
			lastFeedback = fmt.Sprintf("BUILD FAILED: %v\nOutput: %s", err, string(buildOut))
			exec.CommandContext(ctx, "git", "-C", repoPath, "checkout", ".").Run(); continue
		}

		if meta.BenchCommand != "" {
			cmd := exec.CommandContext(ctx, "bash", "-c", meta.BenchCommand)
			cmd.Dir = repoPath
			bOut, _ := cmd.CombinedOutput()
			postBench = string(bOut)
		}

		// Step 5: Post-Action Multi-Agent Review (Critic & Reviewer)
		diffCmd := exec.CommandContext(ctx, "git", "-C", repoPath, "diff", "HEAD")
		diffOut, _ := diffCmd.CombinedOutput()
		finalDiff = string(diffOut)
		o.store.UpdateTaskProposedDiff(taskID, finalDiff)

		reviewInput := fmt.Sprintf("DIFF:\n%s\n\nPRE-BENCH:\n%s\n\nPOST-BENCH:\n%s", finalDiff, preBench, postBench)
		
		// Critic Agent evaluates Risk & Architecture consistency
		criticResp, _ := critic.Process(ctx, reviewInput)
		o.logDeepTechnical(ctx, taskID, "CRITIC", "비판적 아키텍처 및 리스크 검토", reviewInput, criticResp)
		
		if !critic.IsApproved(criticResp) {
			o.logDeepTechnical(ctx, taskID, "REJECTED_CRITIC", "비판적 검토 반려: 리스크 감지됨", criticResp, "")
			lastFeedback = "CRITIC REJECTION: " + criticResp
			exec.CommandContext(ctx, "git", "-C", repoPath, "checkout", ".").Run(); continue
		}

		// Reviewer Agent makes the final functional approval
		reviewResp, _ := reviewer.Process(ctx, reviewInput)
		o.logDeepTechnical(ctx, taskID, "REVIEW", "최종 기능 무결성 검토", reviewInput, reviewResp)

		if !reviewer.IsApproved(reviewResp) {
			o.logDeepTechnical(ctx, taskID, "REJECTED_REVIEWER", "기능 검토 반려: 논리적 오류 발견", reviewResp, "")
			lastFeedback = "REVIEWER REJECTION: " + reviewResp
			exec.CommandContext(ctx, "git", "-C", repoPath, "checkout", ".").Run(); continue
		}

		// Step 6: Human Approval Gate (or Auto-Push if approved)
		if !isApproved {
			o.logDeepTechnical(ctx, taskID, "WAIT", "기술 및 리스크 검증 완료. 최종 승인 대기 중", "", "")
			return RunResult{RepoName: targetRepo, WaitingApproval: true}, nil
		}

		prURL, _ := o.gitMgr.PushApprovedChanges(repoPath, targetRepo, currentBranch, "feat: automated code enhancement with multi-agent consensus")
		o.logDeepTechnical(ctx, taskID, "COMPLETED", "성공적으로 PR을 생성했습니다.", prURL, "")
		return RunResult{RepoName: targetRepo, PRURL: prURL}, nil
	}
	return RunResult{RepoName: targetRepo}, fmt.Errorf("최대 재시도 횟수(3회)를 초과했습니다. Critic 또는 Reviewer가 계속 반려 중입니다.")
}
