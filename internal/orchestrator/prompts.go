package orchestrator

import "fmt"

func getScopeExtractionPrompt(userRequest string) string {
	return fmt.Sprintf(`사용자 요청에서 '대상 레포지토리 명'과 '탐색 대상 폴더/파일 경로'만 추출해줘.
분석 지시나 수정 내용은 제외하고 오직 '어디를 찾아야 하는지'에만 집중해라.

[User Request]
%s

MANDATORY JSON FORMAT:
{"repo": "레포지토리명", "path": "폴더 또는 파일 경로"}`, userRequest)
}

func getInspectionQueryPrompt(userRequest, repo, path, inventory, filesList string) string {
	return fmt.Sprintf(`CIE(Eyes)에게 보낼 '사전 검사(Inspection)' 질의문을 생성해줘.
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
{"inspection_query": "CIE에게 보낼 고정밀 질의문"}`, userRequest, repo, path, inventory, filesList)
}

func getStrategyPrompt(userRequest, inspectRes, inventory string) string {
	return fmt.Sprintf(`사전 검사 결과를 바탕으로 작업 전략을 수립해줘. 

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
`, userRequest, inspectRes, inventory)
}
