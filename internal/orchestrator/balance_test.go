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
// 그 자리만 되돌리고 나머지는 살린다 — 예전에는 저장소 전체가 날아갔다.
func TestBrokenElseIfIsRefusedAlone(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte(elseIfChain), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := applyVariantPlan(root, insightclient.VariantRepoPlan{
		Repo: "r",
		Changes: []insightclient.VariantChange{
			{
				File: "r/a.go", InsertAfter: 6,
				Block:  []string{"\t} else if p.IsInstagram() {", "\t\tt = INSTAGRAM", "\t}"},
				Anchor: "}", AnchorLine: 6,
			},
			{
				File: "r/a.go", InsertAfter: 5,
				Block:  []string{"\t} else if p.IsKakao() {", "\t\tt = KAKAO"},
				Anchor: "t = NAVER", AnchorLine: 5,
			},
		},
	})
	if err != nil {
		t.Fatalf("저장소 전체를 버렸다: %v", err)
	}
	if len(out.Refused) != 1 {
		t.Fatalf("되돌린 자리 %d개: %+v", len(out.Refused), out.Refused)
	}
	if !strings.Contains(out.Refused[0].Why, "균형") {
		t.Errorf("되돌린 까닭: %q", out.Refused[0].Why)
	}
	if out.Inserted != 1 {
		t.Errorf("멀쩡한 자리를 못 살렸다: 넣은 수 %d", out.Inserted)
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
