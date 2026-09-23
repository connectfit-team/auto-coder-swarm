package orchestrator

import (
	"strings"
	"testing"
)

func TestTracedSubsetKeepsOnlyTraced(t *testing.T) {
	seen := map[string]tracedContract{
		"ceoweb/v1/connect.service.ts": {contract: "ceoweb/v1/connect.service.ts", why: "따라감"},
		"ceoweb/v1/worker.service.ts":  {contract: "ceoweb/v1/worker.service.ts", why: "따라감"},
	}
	ev := map[string]string{
		"ceoweb/v1/connect.service.ts": "근거 A",
		"ceoweb/v1/worker.service.ts":  "근거 B",
		"ceoweb/v1/ceo.service.ts":     "근거 C (안 따라간 것)",
	}
	traced := []string{"ceoweb/v1/worker.service.ts", "ceoweb/v1/connect.service.ts"}

	sub, subEv := tracedSubset(traced, seen, ev)
	if len(sub) != 2 {
		t.Fatalf("따라간 둘만 남아야 한다: %v", sub)
	}
	for _, k := range sub {
		if strings.Contains(k, "ceo.service") {
			t.Fatalf("안 따라간 것이 섞였다: %v", sub)
		}
		if subEv[k] == "" {
			t.Fatalf("%s 의 근거가 빠졌다", k)
		}
	}
	// 차례가 못 박혀 있어야 회차마다 답이 안 흔들린다.
	if sub[0] > sub[1] {
		t.Fatalf("차례가 안 정해졌다: %v", sub)
	}
}

func TestTracedSubsetDedupesAndSurvivesMissingEvidence(t *testing.T) {
	seen := map[string]tracedContract{"a.ts": {contract: "a.ts"}}
	sub, subEv := tracedSubset([]string{"a.ts", "a.ts", "없는.ts"}, seen, map[string]string{})
	if len(sub) != 1 || sub[0] != "a.ts" {
		t.Fatalf("겹친 것·없는 것을 못 걸렀다: %v", sub)
	}
	if _, ok := subEv["a.ts"]; !ok {
		t.Fatal("근거가 없어도 자리는 있어야 한다")
	}
}

func TestTracedSubsetEmpty(t *testing.T) {
	sub, _ := tracedSubset(nil, map[string]tracedContract{}, map[string]string{})
	if len(sub) != 0 {
		t.Fatalf("빈 입력에서 무언가 나온다: %v", sub)
	}
}
