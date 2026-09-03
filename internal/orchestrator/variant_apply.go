package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/insightclient"
)

// 계획을 작업공간에 적용한다.
//
// 한 파일에 여러 곳을 넣을 때 아래에서 위로 넣는다. 위에서부터 넣으면
// 아래 줄 번호가 밀려 코드가 한 칸씩 어긋난 자리에 들어가고, 빌드가
// 통과해 버리면 알아채기 어렵다.

// ApplyOutcome 은 한 저장소에 적용한 결과다.
type ApplyOutcome struct {
	Repo     string
	Files    []string
	Inserted int
	Skipped  int // 이미 들어 있어 건너뛴 곳
}

// applyVariantPlan 은 한 저장소의 변경을 작업공간에 적용한다.
func applyVariantPlan(repoRoot string, plan insightclient.VariantRepoPlan) (ApplyOutcome, error) {
	out := ApplyOutcome{Repo: plan.Repo}

	byFile := map[string][]insightclient.VariantChange{}
	for _, c := range plan.Changes {
		byFile[c.File] = append(byFile[c.File], c)
	}
	names := make([]string, 0, len(byFile))
	for f := range byFile {
		names = append(names, f)
	}
	sort.Strings(names)

	for _, name := range names {
		rel := stripRepo(name, plan.Repo)
		path := filepath.Join(repoRoot, rel)
		b, err := os.ReadFile(path)
		if err != nil {
			return out, fmt.Errorf("%s 를 못 읽었다: %w", rel, err)
		}
		lines := strings.Split(string(b), "\n")

		cs := byFile[name]
		sort.Slice(cs, func(i, j int) bool { return cs[i].InsertAfter > cs[j].InsertAfter })

		changed := false
		for _, c := range cs {
			at := c.InsertAfter
			if at < 0 || at > len(lines) {
				return out, fmt.Errorf("%s:%d 는 파일 범위 밖이다(%d줄)", rel, at, len(lines))
			}
			if blockPresent(lines, c.Block) {
				out.Skipped++
				continue
			}
			lines = append(lines[:at], append(append([]string{}, c.Block...), lines[at:]...)...)
			out.Inserted++
			changed = true
		}
		if changed {
			if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
				return out, fmt.Errorf("%s 를 못 썼다: %w", rel, err)
			}
			out.Files = append(out.Files, rel)
		}
	}
	return out, nil
}

// blockPresent 는 그 블록이 이미 파일에 있는지 본다.
// 두 번 돌려도 두 번 들어가지 않아야 한다.
func blockPresent(lines, block []string) bool {
	if len(block) == 0 {
		return true
	}
	want := flattenBlock(block)
	if want == "" {
		return true
	}
	for i := 0; i+len(block) <= len(lines); i++ {
		if flattenBlock(lines[i:i+len(block)]) == want {
			return true
		}
	}
	return false
}

func flattenBlock(ls []string) string {
	var b strings.Builder
	for _, l := range ls {
		b.WriteString(strings.Join(strings.Fields(l), ""))
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}

func stripRepo(path, repo string) string {
	if after, ok := strings.CutPrefix(path, repo+"/"); ok {
		return after
	}
	if i := strings.Index(path, "/"); i > 0 {
		return path[i+1:]
	}
	return path
}
