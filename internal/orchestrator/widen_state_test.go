package orchestrator

import (
	"strings"
	"testing"
)

// 계획한 것을 앞에 두고 후보를 뒤에 붙인다.
//
// stateSheet 이 앞에서부터 담으므로, 계획한 파일이 무언가를 내면 후보는 쓰이지
// 않는다. 앞엣것이 아무것도 못 낼 때만 뒤엣것이 쓰인다 — 실측 W-40774 가
// `tests/crud/connection.spec.ts`(아직 없는 파일) 하나만 짚어 그렇게 됐다.
func TestWidenKeepsPlannedFirst(t *testing.T) {
	got := widenWithCandidates(
		[]string{"tests/crud/connection.spec.ts"},
		[]string{"src/routes/connect/+page.svelte", "src/lib/types/connectinvite.ts"},
	)
	if len(got) != 3 {
		t.Fatalf("%d개: %v", len(got), got)
	}
	if got[0] != "tests/crud/connection.spec.ts" {
		t.Fatalf("계획한 것이 앞이 아니다: %v", got)
	}
}

// 겹치는 것은 한 번만. 같은 파일을 두 번 캐면 목록만 먹는다.
func TestWidenDropsDuplicates(t *testing.T) {
	got := widenWithCandidates(
		[]string{"a.ts", "b.ts", "a.ts"},
		[]string{"b.ts", "c.ts", ""},
	)
	if strings.Join(got, ",") != "a.ts,b.ts,c.ts" {
		t.Fatalf("%v", got)
	}
}

// 후보가 없으면 넓힐 것이 없다 — 부르는 쪽이 len 비교로 거른다.
func TestWidenWithoutCandidatesChangesNothing(t *testing.T) {
	planned := []string{"a.ts"}
	got := widenWithCandidates(planned, nil)
	if len(got) != len(planned) {
		t.Fatalf("후보가 없는데 늘었다: %v", got)
	}
	if len(widenWithCandidates(nil, nil)) != 0 {
		t.Fatal("빈 입력에 무언가 냈다")
	}
}

// 계획한 것이 전부 후보 안에 있으면 늘지 않는다 — 괜히 다시 묻지 않게.
func TestWidenDoesNotGrowWhenAlreadyCovered(t *testing.T) {
	planned := []string{"a.ts", "b.ts"}
	got := widenWithCandidates(planned, []string{"a.ts", "b.ts"})
	if len(got) != len(planned) {
		t.Fatalf("늘면 안 된다: %v", got)
	}
}
