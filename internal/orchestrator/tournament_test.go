package orchestrator

import (
	"strings"
	"testing"
)

func TestParseABFindsTheLetterAlone(t *testing.T) {
	for raw, want := range map[string]string{
		"A": "A", " B ": "B", "a\n": "A", "b.": "B",
		"A 입니다": "A",
		"정답: B": "B",
	} {
		got, ok := parseAB(raw)
		if !ok || got != want {
			t.Fatalf("%q → %q,%v (기대 %q)", raw, got, ok, want)
		}
	}
	for _, raw := range []string{"", "모르겠습니다", "1", "AB"} {
		if got, ok := parseAB(raw); ok {
			t.Fatalf("%q 에서 %q 를 집었다", raw, got)
		}
	}
}

// 「이 계약은…」 같은 답에서 낱말 속 글자를 집으면 안 된다.
func TestParseABIgnoresLettersInsideWords(t *testing.T) {
	if got, ok := parseAB("contract Alpha 가 맞다"); ok {
		t.Fatalf("낱말 속 글자를 집었다: %q", got)
	}
}

// 후보가 하나뿐이면 판을 벌이지 않는다.
func TestTournamentTrivialCases(t *testing.T) {
	tc := &taskContext{}
	if p, _ := tc.tournamentPick(nil, nil); p != "" {
		t.Fatalf("빈 후보에서 무언가 나온다: %q", p)
	}
	p, why := tc.tournamentPick([]string{"only.ts"}, nil)
	if p != "only.ts" || !strings.Contains(why, "하나뿐") {
		t.Fatalf("하나뿐인데 판을 벌였다: %q %q", p, why)
	}
}

func TestTournamentFieldIsCapped(t *testing.T) {
	if maxTournamentCandidates <= 1 || maxTournamentCandidates > 64 {
		t.Fatalf("상한이 말이 안 된다: %d", maxTournamentCandidates)
	}
	// 한 번은 자리를 바꿔 물어야 위치 쏠림이 드러난다.
	if matchVotes < 3 {
		t.Fatalf("자리 바꿔 묻기가 들어갈 자리가 없다: %d", matchVotes)
	}
}
