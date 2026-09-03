package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/connectfit-team/auto-coder-swarm/internal/insightclient"
)

func repoWith(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func planWith(c insightclient.VariantChange) insightclient.VariantRepoPlan {
	return insightclient.VariantRepoPlan{Repo: "r", Changes: []insightclient.VariantChange{c}}
}

// 사본이 뒤처져 줄 번호가 밀렸어도 기준 줄을 보고 제자리에 넣는다.
func TestApplyFollowsMovedAnchor(t *testing.T) {
	root := repoWith(t, "// 새로 생긴 줄\n// 또 하나\nconst naver = 1\nconst apple = 2\n")
	out, err := applyVariantPlan(root, planWith(insightclient.VariantChange{
		File: "r/a.go", InsertAfter: 1, Block: []string{"const instagram = 3"},
		Anchor: "const naver = 1", AnchorLine: 1,
	}))
	if err != nil {
		t.Fatalf("적용 실패: %v", err)
	}
	if out.Inserted != 1 {
		t.Fatalf("넣은 수 = %d", out.Inserted)
	}
	got, _ := os.ReadFile(filepath.Join(root, "a.go"))
	want := "// 새로 생긴 줄\n// 또 하나\nconst naver = 1\nconst instagram = 3\nconst apple = 2\n"
	if string(got) != want {
		t.Errorf("결과:\n%s\n원한 것:\n%s", got, want)
	}
}

// 기준 줄이 사라졌으면 어림짐작으로 넣지 않고 멈춘다.
func TestApplyStopsWhenAnchorGone(t *testing.T) {
	root := repoWith(t, "const apple = 2\n")
	if _, err := applyVariantPlan(root, planWith(insightclient.VariantChange{
		File: "r/a.go", InsertAfter: 1, Block: []string{"const instagram = 3"},
		Anchor: "const naver = 1", AnchorLine: 1,
	})); err == nil || !strings.Contains(err.Error(), "못 찾았다") {
		t.Errorf("오류 내용: %v", err)
	}
}

// 기준 줄이 여러 곳이면 어디인지 모른다 — 멈춘다.
func TestApplyStopsWhenAnchorAmbiguous(t *testing.T) {
	root := repoWith(t, "x\nconst naver = 1\ny\nconst naver = 1\n")
	if _, err := applyVariantPlan(root, planWith(insightclient.VariantChange{
		File: "r/a.go", InsertAfter: 1, Block: []string{"z"},
		Anchor: "const naver = 1", AnchorLine: 99,
	})); err == nil || !strings.Contains(err.Error(), "여러 곳") {
		t.Errorf("오류 내용: %v", err)
	}
}
