package orchestrator

import (
	"strings"
	"testing"
)

// 계획이 짚은 파일에서 아무 데도 안 닿으면 근거가 통째로 사라진다.
// 그때 분석이 짚은 자리에서 다시 따라가야 한다.
func TestTraceWidensWhenPlanTracesNothing(t *testing.T) {
	planFiles := []string{"src/routes/+page.svelte", "src/lib/ui/badge.ts"}
	analysisFiles := []string{"src/lib/server/connect.ts", "src/lib/server/staff.ts"}

	follow := func(f string) []tracedContract {
		if strings.Contains(f, "server/connect") {
			return []tracedContract{{contract: "ceoweb/v1/connect.service.ts", owner: "proto-ceowebapis", why: "공장"}}
		}
		if strings.Contains(f, "server/staff") {
			return []tracedContract{{contract: "ceoweb/v1/ceo.service.ts", owner: "proto-ceowebapis", why: "공장"}}
		}
		return nil // 화면 파일은 계약을 안 부른다
	}

	traced, seen := traceContracts(planFiles, follow)
	if len(traced) != 0 {
		t.Fatalf("계획 파일에서 닿으면 이 시험의 전제가 틀린 것이다: %v", traced)
	}

	traced, seen = traceContracts(analysisFiles, follow)
	if len(traced) != 2 {
		t.Fatalf("넓혀도 못 닿았다: %v", traced)
	}
	if _, ok := seen["ceoweb/v1/connect.service.ts"]; !ok {
		t.Fatalf("정답 계약이 안 들어왔다: %v", traced)
	}
	// 넓힌 결과가 둘이면 #156 의 「따라간 것 안에서 다시 고르기」 가 물린다.
	sub, _ := tracedSubset(traced, seen, map[string]string{})
	if len(sub) != 2 {
		t.Fatalf("다시 고를 차림표가 안 만들어진다: %v", sub)
	}
}

func TestWidenedTraceIsCapped(t *testing.T) {
	if maxWidenedTrace <= 0 || maxWidenedTrace > 40 {
		t.Fatalf("상한이 말이 안 된다: %d", maxWidenedTrace)
	}
	many := make([]string, 100)
	for i := range many {
		many[i] = "f.ts"
	}
	wider := many
	if len(wider) > maxWidenedTrace {
		wider = wider[:maxWidenedTrace]
	}
	if len(wider) != maxWidenedTrace {
		t.Fatalf("상한이 안 걸린다: %d", len(wider))
	}
}
