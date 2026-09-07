package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/connectfit-team/auto-coder-swarm/internal/insightclient"
)

// 계획을 임시 워크트리에 실제로 적용해 본다. PR 은 열지 않는다.
// APPLY_DRY=<계획 json> 으로만 돈다.
func TestApplyDry(t *testing.T) {
	planFile := os.Getenv("APPLY_DRY")
	if planFile == "" {
		t.Skip("APPLY_DRY 없음")
	}
	b, err := os.ReadFile(planFile)
	if err != nil {
		t.Fatal(err)
	}
	var plans []insightclient.VariantRepoPlan
	if err := json.Unmarshal(b, &plans); err != nil {
		t.Fatal(err)
	}

	root := os.Getenv("HOME") + "/cie-repos"
	base := t.TempDir()

	for _, p := range plans {
		if len(p.Changes) == 0 {
			fmt.Printf("\n##### %s — 고칠 자리 없음 (사람이 볼 것 %d개)\n", p.Repo, len(p.NeedsManual))
			continue
		}
		src := p.SourcePath
		if src == "" {
			src = p.Repo
		}
		dst := filepath.Join(base, p.Repo)
		branch := "dry/" + p.Repo

		out, err := exec.Command("git", "-C", filepath.Join(root, src),
			"worktree", "add", "-q", "--detach", dst).CombinedOutput()
		if err != nil {
			fmt.Printf("\n##### %s — 워크트리 실패: %s\n", p.Repo, out)
			continue
		}

		res, err := applyVariantPlan(dst, p)
		if err != nil {
			fmt.Printf("\n##### %s — 적용 실패: %v\n", p.Repo, err)
		} else {
			msg := verifyRepo(dst, res.Files, res.Unformatted)
			fmt.Printf("\n##### %s — 넣음 %d · 건너뜀 %d · 검증 %s\n",
				p.Repo, res.Inserted, res.Skipped, orOK(msg))
			d, _ := exec.Command("git", "-C", dst, "diff", "--stat").CombinedOutput()
			fmt.Print(string(d))
		}
		exec.Command("git", "-C", filepath.Join(root, src), "worktree", "remove", "--force", dst).Run()
		_ = branch
	}
}

func orOK(msg string) string {
	if msg == "" {
		return "통과"
	}
	return "실패: " + msg
}
