package orchestrator

import (
	"encoding/json"
	"fmt"
	"strings"
	"log"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
)

func (t *taskContext) prepareAnalysis() error {
	if t.analysis != "" {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "INIT", "분석 컨텍스트 주입됨", "", t.analysis)
		return nil
	}

	sessionID := fmt.Sprintf("swarm-task-%s", t.taskID)

	// [Step 52: Chain Reaction Context]
	if t.req.Depth > 0 {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CHAIN_CONTEXT", 
			fmt.Sprintf("연쇄 작업 진행 중 (남은 Depth: %d)", t.req.Depth), "", "")
	}

	// [Step 0-1: Scope Extraction]
	scopePrompt := fmt.Sprintf(`사용자 요청에서 '대상 레포지토리 명'과 '탐색 대상 폴더/파일 경로'만 추출해줘.
분석 지시나 수정 내용은 제외하고 오직 '어디를 찾아야 하는지'에만 집중해라.

[User Request]
%s

MANDATORY JSON FORMAT:
{"repo": "레포지토리명", "path": "폴더 또는 파일 경로"}`, t.req.UserRequest)

	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "SCOPE_EXTRACTION", "분석 범위(Repo/Path) 추출 중", scopePrompt, "")
	scopeRaw, err := agent.CallLLM(t.ctx, t.primaryLLM, "ScopeExtractor", scopePrompt)
	if err != nil {
		log.Printf("[Orchestrator] Scope extraction LLM failed: %v", err)
		return fmt.Errorf("scope extraction failed: %w", err)
	}

	var scope struct {
		Repo string `json:"repo"`
		Path string `json:"path"`
	}
	json.Unmarshal([]byte(extractJSON(scopeRaw)), &scope)

	// [Risk Mitigation] Fallback logic
	if scope.Repo == "" {
		scope.Repo = t.req.TargetRepo
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "SCOPE_FALLBACK", "Repo 추출 실패, 요청된 기본 Repo 사용", "", scope.Repo)
	}
	if scope.Path == "" {
		scope.Path = "전체"
	}

	if scope.Repo == "" {
		err := fmt.Errorf("대상 레포지토리를 특정할 수 없습니다. 사용자 요청에 레포지토리 명을 명시해주세요.")
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "ERROR", "검사 범위 파악 실패", scopeRaw, "")
		return err
	}
	t.targetRepo = scope.Repo

	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "SCOPE_RESOLVED", fmt.Sprintf("탐색 대상 확정 - Repo: %s, Path: %s", scope.Repo, scope.Path), "", "")

	// [Step 0-1-B: High-Precision Introspection]
	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "FETCHING_INVENTORY", "CIE 인벤토리 및 파일 목록 조회 중", "", "")
	inventory, err := t.orchestrator.insightClient.GetRepoInventory(t.ctx, scope.Repo)
	if err != nil {
		log.Printf("[Orchestrator] Failed to fetch inventory for %s: %v", scope.Repo, err)
	}

	// Intelligent extension detection
	ext := ""
	lowerReq := strings.ToLower(t.req.UserRequest)
	langMap := map[string]string{
		"dart": "dart", "flutter": "dart",
		"go": "go", "golang": "go",
		"python": "py", "py": "py",
		"javascript": "js", "node": "js", "js": "js",
		"typescript": "ts", "ts": "ts",
	}
	for key, val := range langMap {
		if strings.Contains(lowerReq, key) {
			ext = val
			break
		}
	}

	files, _ := t.orchestrator.insightClient.GetRepoFiles(t.ctx, scope.Repo, ext, 3)
	inventoryJSON, _ := json.MarshalIndent(inventory, "", "  ")
	filesList := strings.Join(files, "\n")

	// [Step 0-2: Inspection Query Generation]
	inspectionGeneratorPrompt := fmt.Sprintf(`CIE(Eyes)에게 보낼 '사전 검사(Inspection)' 질의문을 생성해줘.
대상 범위의 '파일 목록'과 '코드 규모(LOC)'를 빠르게 파악하는 것이 목적이다.

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
	inspectQueryRaw, err := agent.CallLLM(t.ctx, t.primaryLLM, "QueryArchitect", inspectionGeneratorPrompt)
	if err != nil {
		return fmt.Errorf("inspection query generation failed: %w", err)
	}

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

	// [Step 1: Task Strategy]
	strategyPrompt := fmt.Sprintf(`사전 검사 결과를 바탕으로 작업 전략을 수립해줘. 

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
	stratRaw, err := agent.CallLLM(t.ctx, t.primaryLLM, "Architect", strategyPrompt)
	if err != nil {
		return fmt.Errorf("strategy generation failed: %w", err)
	}

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
