package orchestrator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// 문제 목록의 S-03 — 원래 깨진 시험 파일이 저장소를 잠그면 안 된다.
//
// 목록에 적힌 사다리가 실제 표(projectDefaults)와 같은지 본다. 표만 고치고
// 목록을 안 고치면 다음 사람이 무엇이 지켜지는지 알 수 없다.
func TestBaselineLadder(t *testing.T) {
	type ladderProblem struct {
		ID     string              `json:"id"`
		Ladder map[string][]string `json:"ladder"`
	}
	var found bool
	for _, p := range scopeProblemsRaw(t) {
		var lp ladderProblem
		if err := json.Unmarshal(p, &lp); err != nil || len(lp.Ladder) == 0 {
			continue
		}
		found = true
		for kind, want := range lp.Ladder {
			got := ladderFor(kind)
			if len(got) != len(want) {
				t.Errorf("%s: %s 사다리가 %d칸인데 목록은 %d칸이다 — %v",
					lp.ID, kind, len(got), len(want), got)
				continue
			}
			for i := range want {
				if got[i] != want[i] {
					t.Errorf("%s: %s 사다리 %d칸이 다르다\n  코드: %q\n  목록: %q",
						lp.ID, kind, i+1, got[i], want[i])
				}
			}
		}
	}
	if !found {
		t.Skip("목록에 사다리 문항이 없다")
	}
}

// ladderFor 는 그 종류의 검증 명령 사다리를 준다(엄한 것부터).
func ladderFor(kind string) []string {
	for _, d := range projectDefaults {
		if d.kind == kind {
			return append([]string{d.build}, d.weaker...)
		}
	}
	return nil
}

func scopeProblemsRaw(t *testing.T) []json.RawMessage {
	t.Helper()
	paths := []string{os.Getenv("PROBLEMS_PATH"),
		filepath.Join("..", "..", "..", "code-insight-engine", "internal", "business", "testdata", "problems.json"),
		filepath.Join(os.Getenv("HOME"), "projects", "code-insight-engine", "internal", "business", "testdata", "problems.json"),
	}
	for _, p := range paths {
		if p == "" {
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var all struct {
			Scope []json.RawMessage `json:"scope"`
		}
		if err := json.Unmarshal(b, &all); err != nil {
			t.Fatalf("%s 를 읽지 못했다: %v", p, err)
		}
		return all.Scope
	}
	t.Skip("문제 목록을 못 찾았다 — PROBLEMS_PATH 로 알려 주면 본다")
	return nil
}
