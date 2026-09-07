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

// 한 저장소의 실제 diff 를 본다. SHOW_REPO 로만 돈다.
func TestShowDiff(t *testing.T) {
	want := os.Getenv("SHOW_REPO")
	if want == "" {
		t.Skip("SHOW_REPO 없음")
	}
	b, _ := os.ReadFile(os.Getenv("APPLY_DRY"))
	var plans []insightclient.VariantRepoPlan
	_ = json.Unmarshal(b, &plans)

	root := os.Getenv("HOME") + "/cie-repos"
	base := t.TempDir()
	for _, p := range plans {
		if p.Repo != want {
			continue
		}
		src := p.SourcePath
		if src == "" {
			src = p.Repo
		}
		dst := filepath.Join(base, p.Repo)
		exec.Command("git", "-C", filepath.Join(root, src), "worktree", "add", "-q", "--detach", dst).Run()
		if _, err := applyVariantPlan(dst, p); err != nil {
			fmt.Println("적용 실패:", err)
		}
		d, _ := exec.Command("git", "-C", dst, "diff", "-U2").CombinedOutput()
		fmt.Print(string(d))
		exec.Command("git", "-C", filepath.Join(root, src), "worktree", "remove", "--force", dst).Run()
	}
}
