package orchestrator

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
)

func (t *taskContext) prepareAnalysis() error {
	if t.analysis != "" {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "INIT", "분석 컨텍스트 주입됨", "", t.analysis)
		// 분석을 건너뛰는 경로에서도 절차는 받아야 한다. 여기서 빠뜨리면
		// 연쇄 작업(Chain Reaction)만 규약 없이 코드를 쓴다.
		t.fetchSkills(t.req.TargetRepo, extsFromRequest(t.req.UserRequest))
		return nil
	}

	sessionID := fmt.Sprintf("swarm-task-%s", t.taskID)

	// [Step 52: Chain Reaction Context]
	if t.req.Depth > 0 {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CHAIN_CONTEXT",
			fmt.Sprintf("연쇄 작업 진행 중 (남은 Depth: %d)", t.req.Depth), "", "")
	}

	// [Step 0-1: Scope Extraction]
	scopePrompt := getScopeExtractionPrompt(t.req.UserRequest)
	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "SCOPE_EXTRACTION", "분석 범위(Repo/Path) 추출 중", scopePrompt, "")

	var scope struct {
		Repo string `json:"repo"`
		Path string `json:"path"`
	}
	scopeRaw, err := agent.CallLLMJSON(t.ctx, t.primaryLLM, "ScopeExtractor", scopePrompt, &scope)
	if err != nil {
		// 범위는 요청에 적힌 저장소로 물러설 수 있다. 여기서 죽이지 않는다.
		log.Printf("[Orchestrator] 범위 추출 실패 (요청의 target_repo 로 물러섬): %v", err)
	}

	if scope.Repo == "" {
		scope.Repo = t.req.TargetRepo
	}
	if scope.Path == "" {
		scope.Path = "전체"
	}

	if scope.Repo == "" {
		err := fmt.Errorf("대상 레포지토리를 특정할 수 없습니다.")
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "ERROR", "검사 범위 파악 실패", scopeRaw, "")
		return err
	}
	t.targetRepo = scope.Repo

	// [Team Skills: 작업 절차] 전략·계획 프롬프트가 이걸 타고 간다.
	t.fetchSkills(scope.Repo, extsFromRequest(t.req.UserRequest))

	// [CKH Integration: Corporate Policy Retrieval]
	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "KNOWLEDGE_RETRIEVAL", "사내 정책 및 관련 지식 조회 중 (CKH)", "", "")
	ckhRes, err := t.orchestrator.ckhClient.GetContextReport(t.ctx, t.taskID, t.req.UserRequest, t.targetRepo)
	if err == nil && ckhRes != nil {
		t.ckhKnowledge = ckhRes.Summary
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "KNOWLEDGE_FOUND", "사내 지식 확보 완료", "", t.ckhKnowledge)
	} else {
		log.Printf("[Orchestrator] Failed to fetch CKH report: %v", err)
	}

	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "SCOPE_RESOLVED", fmt.Sprintf("탐색 대상 확정 - Repo: %s, Path: %s", scope.Repo, scope.Path), "", "")

	// [Step 0-1-B: High-Precision Introspection]
	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "FETCHING_INVENTORY", "CIE 인벤토리 조회 중", "", "")
	inventory, _ := t.orchestrator.insightClient.GetRepoInventory(t.ctx, scope.Repo)

	ext := t.detectExtension()
	files, _ := t.orchestrator.insightClient.GetRepoFiles(t.ctx, scope.Repo, ext, 3)

	inventoryJSON, _ := json.Marshal(inventory)
	inventoryJSONStr := string(inventoryJSON)
	inventoryRunes := []rune(inventoryJSONStr)
	if len(inventoryRunes) > 5000 {
		inventoryJSONStr = string(inventoryRunes[:5000]) + "\n... (생략됨: 인벤토리가 너무 큽니다) ..."
	}

	filesList := strings.Join(files, "\n")
	filesRunes := []rune(filesList)
	if len(filesRunes) > 3000 {
		filesList = string(filesRunes[:3000]) + "\n... (생략됨: 파일 목록이 너무 큽니다) ..."
	}

	// [Step 0-2: Inspection Query Generation]
	inspectionGeneratorPrompt := getInspectionQueryPrompt(t.req.UserRequest, scope.Repo, scope.Path, inventoryJSONStr, filesList)
	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "DETECTION_RAW", "고정밀 사전 검사 질의 생성 중", inspectionGeneratorPrompt, "")

	var inspectQuery struct {
		Query string `json:"inspection_query"`
	}
	// 질의문은 아래에 폴백이 있다. 형식이 끝내 안 맞아도 진행한다.
	if _, err := agent.CallLLMJSON(t.ctx, t.primaryLLM, "QueryArchitect", inspectionGeneratorPrompt, &inspectQuery); err != nil {
		log.Printf("[Orchestrator] 사전 검사 질의 생성 실패 (기본 질의로 물러섬): %v", err)
	}
	if inspectQuery.Query == "" {
		inspectQuery.Query = fmt.Sprintf("[%s] 레포지토리의 [%s] 경로 아래 파일 목록과 LOC를 알려줘.", scope.Repo, scope.Path)
	}

	onWorkID := func(wid string) { t.orchestrator.store.UpdateCIEWorkID(t.taskID, wid) }
	inspectRes, _, err := t.orchestrator.insightClient.QueryOracle(t.ctx, inspectQuery.Query, sessionID, onWorkID)
	if err != nil {
		return err
	}

	// [Step 1: Task Strategy]
	// **찾아서 준다.** 잘린 목록을 보여 주고 고르게 하면 큰 저장소에서는
	// 제비뽑기가 된다 — cms 1130개 중 일부만 보여 주고 있었고, "월별 근무
	// 조회 말일 누락" 에 이름이 비슷한 workstamp.ts 를 골랐다.
	candidates := ""
	if cands, err := t.orchestrator.insightClient.FindCandidates(t.ctx, scope.Repo, t.req.UserRequest, 20); err == nil && len(cands) > 0 {
		var b strings.Builder
		b.WriteString("[요청과 관련된 파일 — 요청의 낱말이 몇 개 걸렸는지 순]\n")
		for _, c := range cands {
			fmt.Fprintf(&b, "%s (낱말 %d개)\n", c.Path, c.Hits)
		}
		b.WriteString("**actionable_path 는 되도록 이 목록에서 고를 것.**\n\n")
		candidates = b.String()
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CANDIDATES",
			fmt.Sprintf("관련 파일 %d개 확보", len(cands)), "", candidates)
	}

	strategyPrompt := getStrategyPrompt(t.req.UserRequest, inspectRes, string(inventoryJSON), candidates)
	// Inject CKH Knowledge into Strategy
	if t.ckhKnowledge != "" {
		strategyPrompt = fmt.Sprintf("[CORPORATE POLICIES & CONTEXT]\n%s\n\n%s", t.ckhKnowledge, strategyPrompt)
	}

	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "STRATEGY", "전략 수립 중", strategyPrompt, "")

	// 파싱 실패를 "작업 규모 과다" 로 보고하지 않는다.
	//
	// IsFeasible 의 zero value 가 false 라, 모델이 JSON 이 아닌 것을 뱉으면
	// **판단해 보지도 않고** 작업이 규모 과다로 죽는다. 원인이 프롬프트 형식인지
	// 실제 규모인지 구별이 안 돼 한참 헤맸다.
	var strategy TaskStrategy
	stratRaw, err := agent.CallLLMJSON(t.ctx, t.primaryLLM, "Architect", strategyPrompt, &strategy)
	if err != nil {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "STRATEGY_UNPARSED", "전략 응답이 JSON 이 아니다", "", stratRaw)
		return fmt.Errorf("전략 수립 실패: %w", err)
	}
	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "STRATEGY_PARSED",
		fmt.Sprintf("파일 %d개 · 가능=%v", strategy.TotalFiles, strategy.IsFeasible), "", stratRaw)
	// **비어 있는 전략과 "못 하겠다" 는 다르다.**
	//
	// 봉투에 싸여 온 응답이 오류 없이 빈 구조체로 읽히면 IsFeasible 이 false 가
	// 되어, 모델이 제대로 답했는데도 규모 과다로 죽었다. 무엇이 비었는지
	// 그대로 말해야 다음 사람이 헤매지 않는다.
	if len(strategy.ActionablePath) == 0 && strategy.TotalFiles == 0 {
		return fmt.Errorf("전략이 비어 있다 — 고칠 파일을 하나도 지목하지 못했다")
	}
	if !strategy.IsFeasible {
		return fmt.Errorf("모델이 작업 규모 과다로 판단했다: %s", strategy.ComplexityRisk)
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

func (t *taskContext) detectExtension() string {
	return detectExtension(t.req.UserRequest)
}

// extsFromRequest 는 요청문에서 짐작한 확장자다. 없으면 빈 목록 —
// 계획이 나오면 실제 파일 경로로 다시 받는다.
func extsFromRequest(req string) []string {
	if e := detectExtension(req); e != "" {
		return []string{e}
	}
	return nil
}

func detectExtension(req string) string {
	lowerReq := strings.ToLower(req)
	langMap := map[string]string{
		"dart": "dart", "flutter": "dart", "go": "go", "golang": "go", "python": "py",
		"javascript": "js", "node": "js", "js": "js", "typescript": "ts", "ts": "ts",
	}
	for key, val := range langMap {
		if strings.Contains(lowerReq, key) {
			return val
		}
	}
	return ""
}
