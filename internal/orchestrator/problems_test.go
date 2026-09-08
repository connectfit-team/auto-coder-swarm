package orchestrator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// 문제 목록은 code-insight-engine 이 들고 있다(측정판이 모여 있는 곳이다).
// 여기서는 이 저장소가 맡은 문항만 골라 본다 — 목록을 둘로 나누면 무엇이
// 남았는지 다시 알 수 없게 된다.

type scopeProblem struct {
	ID       string   `json:"id"`
	Seen     string   `json:"seen"`
	Symptom  string   `json:"symptom"`
	Cause    string   `json:"cause"`
	Names    []string `json:"names"`
	Commands []string `json:"commands"`
	Expect   string   `json:"expect"`
}

func loadScopeProblems(t *testing.T) []scopeProblem {
	t.Helper()
	paths := []string{os.Getenv("PROBLEMS_PATH")}
	paths = append(paths,
		filepath.Join("..", "..", "..", "code-insight-engine", "internal", "business", "testdata", "problems.json"),
		filepath.Join(os.Getenv("HOME"), "projects", "code-insight-engine", "internal", "business", "testdata", "problems.json"),
	)
	for _, p := range paths {
		if p == "" {
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var all struct {
			Scope []scopeProblem `json:"scope"`
		}
		if err := json.Unmarshal(b, &all); err != nil {
			t.Fatalf("%s 를 읽지 못했다: %v", p, err)
		}
		return all.Scope
	}
	t.Skip("문제 목록을 못 찾았다 — PROBLEMS_PATH 로 알려 주면 본다")
	return nil
}

// 모델이 낸 이름·명령을 그대로 쓰면 안 된다.
func TestScopeProblems(t *testing.T) {
	m := newFakeRepos("worker", "ceo", "cms", "gig_mobile", "attendance-api")

	for _, p := range loadScopeProblems(t) {
		for _, name := range p.Names {
			if m.HasRepo(name) {
				t.Errorf("%s: %q 를 저장소로 받아들였다 — %s", p.ID, name, p.Expect)
			}
		}
		for _, cmd := range p.Commands {
			if !isNoCommand(cmd) {
				t.Errorf("%s: %q 를 명령으로 받아들였다 — %s", p.ID, cmd, p.Expect)
			}
		}
	}

	// 진짜 이름과 진짜 명령은 통과해야 한다. 안 그러면 위의 검사는
	// "무엇이든 거절한다" 로도 통과한다.
	for _, name := range []string{"worker", "ceo", "cms", "gig_mobile", "attendance-api"} {
		if !m.HasRepo(name) {
			t.Errorf("있는 저장소를 없다고 했다: %s", name)
		}
	}
	for _, cmd := range []string{"go build ./...", "npm run build", "flutter analyze", "make build"} {
		if isNoCommand(cmd) {
			t.Errorf("멀쩡한 명령을 '없다' 로 봤다: %s", cmd)
		}
	}
}
