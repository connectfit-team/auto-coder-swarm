package orchestrator

import (
	"strings"
	"testing"
)

// W-70980 의 실제 분석문. 이것으로 코드를 쓰면 안 된다.
const realNotFound = `### 분석 결과

**연결 보류 기능 관련 코드 검색 결과:**

1. **` + "`internal_v2/business/work.go`" + ` 파일:**
   - WorkRepository 인터페이스가 정의되어 있습니다.

**확인하지 못한 내용:**
- "연결 보류", "pending connection", "hold connection" 키워드로는 관련 코드를 찾지 못했습니다.
- 구체적인 "연결 보류" 기능 자체에 대한 구현 코드나 파일은 확인하지 못했습니다.
`

func TestAnalysisSaysNotFound(t *testing.T) {
	why, ok := AnalysisSaysNotFound(realNotFound)
	if !ok {
		t.Fatal("분석이 못 찾았다고 했는데 그대로 진행한다 — 손이 지어낸다")
	}
	if !strings.Contains(why, "찾지 못했") && !strings.Contains(why, "확인하지 못했") {
		t.Errorf("까닭을 못 뽑았다: %q", why)
	}
}

// 멀쩡한 분석을 세우면 안 된다. 그러면 할 수 있는 일도 못 한다.
func TestAnalysisFoundKeepsGoing(t *testing.T) {
	good := `### 분석 결과

말일 경계 계산이 lt 를 쓰고 있어 말일이 통째로 빠집니다.

🔗 짚은 것들의 정의 위치(검색으로 확인함):
- ` + "`monthlyRange`" + ` — cms/src/lib/server/workplace.ts:88
`
	if why, ok := AnalysisSaysNotFound(good); ok {
		t.Errorf("멀쩡한 분석을 세웠다: %q", why)
	}

	// 한 줄만 걸리고 찾은 것도 있으면 부분적으로 찾은 것이다.
	partial := `일부 경로는 확인하지 못했습니다.

🔗 짚은 것들의 정의 위치(검색으로 확인함):
- ` + "`WorkRepository`" + ` — ceo/internal_v2/business/work.go:14
`
	if _, ok := AnalysisSaysNotFound(partial); ok {
		t.Error("한 줄 걸린 것으로 작업을 세웠다 — 부분적으로는 찾았다")
	}

	if _, ok := AnalysisSaysNotFound(""); ok {
		t.Error("빈 분석을 '못 찾았다' 로 읽었다")
	}
}
