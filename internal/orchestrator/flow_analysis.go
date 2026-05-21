package orchestrator

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
)

func (t *taskContext) prepareAnalysis() error {
	if t.analysis != "" {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "INIT", "분석 컨텍스트 주입됨", "", t.analysis)
		return nil
	}

	sessionID := fmt.Sprintf("swarm-task-%s", t.taskID)

	// [Step 0-1: Scope Extraction] Extract target repo and path from User Request
	scopePrompt := fmt.Sprintf(`사용자 요청에서 '대상 레포지토리 명'과 '탐색 대상 폴더/파일 경로'만 추출해줘.
분석 지시나 수정 내용은 제외하고 오직 '어디를 찾아야 하는지'에만 집중해라.

[User Request]
%s

MANDATORY JSON FORMAT:
{"repo": "레포지토리명", "path": "폴더 또는 파일 경로"}`, t.req.UserRequest)

	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "SCOPE_EXTRACTION", "분석 범위(Repo/Path) 추출 중", scopePrompt, "")
	scopeRaw, _ := agent.CallLLM(t.ctx, t.primaryLLM, "ScopeExtractor", scopePrompt)

	var scope struct {
		Repo string `json:"repo"`
		Path string `json:"path"`
	}
	json.Unmarshal([]byte(extractJSON(scopeRaw)), &scope)

	// [Risk Mitigation] Fallback to explicitly provided target_repo if LLM extraction fails
	if scope.Repo == "" {
		scope.Repo = t.req.TargetRepo
	}
	if scope.Path == "" {
		scope.Path = "전체" // Default to the whole repository if no specific path is found
	}

	if scope.Repo == "" {
		err := fmt.Errorf("대상 레포지토리를 특정할 수 없습니다. 사용자 요청에 레포지토리 명을 명시해주세요.")
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "ERROR", "검사 범위 파악 실패", scopeRaw, "")
		return err
	}

	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "SCOPE_RESOLVED", fmt.Sprintf("탐색 대상 확정 - Repo: %s, Path: %s", scope.Repo, scope.Path), "", "")

	// [Step 0-1-B: High-Precision Introspection] Fetch Inventory & Files
	inventory, _ := t.orchestrator.insightClient.GetRepoInventory(t.ctx, scope.Repo)
	
	// Intelligently decide extension to filter based on user request (simple heuristic for now)
	ext := ""
	if strings.Contains(strings.ToLower(t.req.UserRequest), "dart") { ext = "dart" }
	if strings.Contains(strings.ToLower(t.req.UserRequest), "go") { ext = "go" }
	
	files, _ := t.orchestrator.insightClient.GetRepoFiles(t.ctx, scope.Repo, ext, 3)
	
	inventoryJSON, _ := json.MarshalIndent(inventory, "", "  ")
	filesList := strings.Join(files, "\n")

	// [Step 0-2: Intelligence-First Inspection Query Generation]
	inspectionGeneratorPrompt := fmt.Sprintf(`CIE(Eyes)에게 보낼 '사전 검사(Inspection)' 질의문을 생성해줘.
이 단계의 목적은 작업을 수행하기 전 대상 범위의 '파일 목록'과 '코드 규모(LOC)'만 빠르게 파악하는 것이다.
제공된 [Repository Inventory] 정보를 참고하여, 분석이 필요한 핵심 타겟을 명확히 명시해라.

[User Request]
%s

[Target Scope]
Repo: %s, Path: %s

[Repository Inventory]
%s

[Initial File List (Sample)]
%s

MANDATORY JSON FORMAT:
{"inspection_query": "CIE에게 보낼 고정밀 질의문"}`, t.req.UserRequest, scope.Repo, scope.Path, string(inventoryJSON), filesList)

	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "DETECTION_RAW", "고정밀 사전 검사 질의 생성 중", inspectionGeneratorPrompt, "")
	inspectQueryRaw, _ := agent.CallLLM(t.ctx, t.primaryLLM, "QueryArchitect", inspectionGeneratorPrompt)
	
	var inspectQuery struct {
		Query string `json:"inspection_query"`
	}
	json.Unmarshal([]byte(extractJSON(inspectQueryRaw)), &inspectQuery)

	if inspectQuery.Query == "" {
		inspectQuery.Query = fmt.Sprintf("[%s] 레포지토리의 [%s] 경로 아래 파일 목록과 LOC를 알려줘.", scope.Repo, scope.Path)
	}

	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "INSPECTION", "고정밀 사전 검사 요청", inspectQuery.Query, "")

	onWorkID := func(wid string) {
		t.orchestrator.store.UpdateCIEWorkID(t.taskID, wid)
	}

	inspectRes, _, err := t.orchestrator.insightClient.QueryOracle(t.ctx, inspectQuery.Query, sessionID, onWorkID)
	if err != nil {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "ERROR", "사전 검사 실패", err.Error(), "")
		return err
	}

	// [Step 1: Task Strategy] Assess scale and refine the analysis query
	strategyPrompt := fmt.Sprintf(`사전 검사 결과를 바탕으로 작업 전략을 수립해줘. 
CIE(Eyes)에게는 '코드의 논리적 이해'와 '기술적 요약'만 요청해야 한다.

[User Request]
%s

[Inspection Result]
%s

[Repository Inventory]
%s

MANDATORY JSON FORMAT:
{
  "total_files": 0,
  "total_lines": 0,
  "complexity_risk": "...",
  "actionable_path": ["..."],
  "analysis_query": "CIE(Eyes)에게 보낼 고정밀 '이해(Analysis)' 질의문",
  "is_feasible": true
}
`, t.req.UserRequest, inspectRes, string(inventoryJSON))

	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "STRATEGY", "전략 수립 중", strategyPrompt, "")
	stratRaw, _ := agent.CallLLM(t.ctx, t.primaryLLM, "Architect", strategyPrompt)

	var strategy TaskStrategy
	json.Unmarshal([]byte(extractJSON(stratRaw)), &strategy)

	if !strategy.IsFeasible {
		return fmt.Errorf("작업 규모 과다 (%d 파일). 작업을 나누어 요청해주세요.", strategy.TotalFiles)
	}

	// [Step 2: Precision Logic Analysis (Eyes)]
	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "ORACLE", "CIE 고정밀 논리 분석 요청", strategy.AnalysisQuery, "")
	res, _, err := t.orchestrator.insightClient.QueryOracle(t.ctx, strategy.AnalysisQuery, sessionID, onWorkID)
	if err != nil {
		return err
	}
	t.analysis = res
	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "INIT", "분석 완료", "", t.analysis)
	return nil
}

