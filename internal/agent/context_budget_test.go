package agent

import (
	"fmt"
	"strings"
	"testing"
)

// 실제로 쌓여 있던 기록의 모양이다 — 같은 줄이 시도마다 되풀이된다.
func realState(rounds int) string {
	var b strings.Builder
	stages := []string{
		"SCOPE_EXTRACTION: 분석 범위(Repo/Path) 추출 중",
		"KNOWLEDGE_RETRIEVAL: 사내 정책 및 관련 지식 조회 중 (CKH)",
		"SCOPE_RESOLVED: 탐색 대상 확정 - Repo: ceo, Path: internal/common/twilio.go",
		"FETCHING_INVENTORY: CIE 인벤토리 조회 중",
		"DETECTION_RAW: 고정밀 사전 검사 질의 생성 중",
		"STRATEGY: 전략 수립 중",
	}
	for r := 0; r < rounds; r++ {
		for i, s := range stages {
			fmt.Fprintf(&b, "[%02d:%02d:00] %s\n", 11+r, i, s)
		}
	}
	return b.String()
}

func TestCompactDropsRepeats(t *testing.T) {
	got := CompactContext(realState(8), 4000)
	if n := strings.Count(got, "FETCHING_INVENTORY"); n != 1 {
		t.Fatalf("되풀이가 남았다: FETCHING_INVENTORY %d번", n)
	}
	if !strings.Contains(got, "twilio.go") {
		t.Fatal("내용이 사라졌다 — 되풀이만 걷어내야 한다")
	}
	if len(got) > len(realState(1))+16 {
		t.Fatalf("여덟 바퀴가 한 바퀴로 줄지 않았다: %d자", len(got))
	}
}

func TestCompactKeepsTheLatest(t *testing.T) {
	state := "[11:00:00] STRATEGY_PARSED: 파일 0개 · 가능=false\n" +
		"[12:00:00] STRATEGY_PARSED: 파일 3개 · 가능=true"
	got := CompactContext(state, 4000)
	if strings.Contains(got, "0개") {
		t.Fatal("옛 결과가 남았다 — 같은 단계는 마지막 것만 남아야 한다")
	}
	if !strings.Contains(got, "3개") {
		t.Fatal("마지막 결과가 사라졌다")
	}
}

func TestCompactHonorsBudget(t *testing.T) {
	// 줄마다 내용이 달라 되풀이로는 줄지 않는다 — 예산이 물어야 한다.
	var b strings.Builder
	for i := 0; i < 400; i++ {
		fmt.Fprintf(&b, "[11:00:00] STAGE%d: 무언가 %d\n", i, i)
	}
	got := CompactContext(b.String(), 1200)
	if len(got) > 1200 {
		t.Fatalf("예산을 넘었다: %d자", len(got))
	}
	// 잘라도 최근 것이 남아야 한다.
	if !strings.Contains(got, "STAGE399") {
		t.Fatal("가장 최근 줄이 잘려 나갔다")
	}
	if strings.Contains(got, "STAGE0:") {
		t.Fatal("가장 오래된 줄이 남았다 — 뒤에서부터 담아야 한다")
	}
}

func TestCompactEmpty(t *testing.T) {
	if CompactContext("", 1200) != "" {
		t.Fatal("빈 기록은 빈 것이어야 한다")
	}
	if CompactContext(realState(2), 0) != "" {
		t.Fatal("예산이 0이면 아무것도 넣지 않아야 한다")
	}
}
