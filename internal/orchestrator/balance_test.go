package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/connectfit-team/auto-coder-swarm/internal/insightclient"
)

const elseIfChain = `func f() {
	if p.IsGoogle() {
		t = GOOGLE
	} else if p.IsNaver() {
		t = NAVER
	}
}
`

// } else if 블록을 닫는 괄호까지 통째로 복사해 뒤에 넣으면 괄호가 어긋난다.
// 파서가 없어도 이것으로 잡힌다 — Dart·TypeScript 도 마찬가지다.
func TestBrokenElseIfIsCaughtByBalance(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte(elseIfChain), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := applyVariantPlan(root, insightclient.VariantRepoPlan{
		Repo: "r",
		Changes: []insightclient.VariantChange{{
			File: "r/a.go", InsertAfter: 6,
			Block:  []string{"\t} else if p.IsInstagram() {", "\t\tt = INSTAGRAM", "\t}"},
			Anchor: "}", AnchorLine: 6,
		}},
	})
	if err == nil {
		t.Fatal("괄호가 어긋났는데 그냥 넣었다")
	}
	if !strings.Contains(err.Error(), "균형") {
		t.Errorf("오류 내용: %v", err)
	}
}

// 성한 블록은 균형이 그대로다.
func TestSoundBlockPasses(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte(elseIfChain), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := applyVariantPlan(root, insightclient.VariantRepoPlan{
		Repo: "r",
		Changes: []insightclient.VariantChange{{
			File: "r/a.go", InsertAfter: 5,
			Block:  []string{"\t} else if p.IsInstagram() {", "\t\tt = INSTAGRAM"},
			Anchor: "t = NAVER", AnchorLine: 5,
		}},
	})
	if err != nil {
		t.Fatalf("성한 블록을 막았다: %v", err)
	}
	if out.Inserted != 1 {
		t.Errorf("넣은 수 = %d", out.Inserted)
	}
}

// 문자열 안의 괄호 때문에 원래 균형이 0 이 아니어도 통과해야 한다.
func TestUnbalancedStringsStillPass(t *testing.T) {
	root := t.TempDir()
	body := "const a = \"{\"\nconst naver = 1\n"
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := applyVariantPlan(root, insightclient.VariantRepoPlan{
		Repo: "r",
		Changes: []insightclient.VariantChange{{
			File: "r/a.go", InsertAfter: 2, Block: []string{"const instagram = 2"},
			Anchor: "const naver = 1", AnchorLine: 2,
		}},
	}); err != nil {
		t.Errorf("문자열 속 괄호 때문에 막혔다: %v", err)
	}
}
