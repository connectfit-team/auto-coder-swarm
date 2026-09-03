package orchestrator

import (
	"fmt"
	"os"
	"os/exec"
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
			shift, err := anchorShift(lines, c)
			if err != nil {
				return out, fmt.Errorf("%s: %w", rel, err)
			}
			at := c.InsertAfter + shift
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
			// 정렬된 var·const 묶음에 한 줄을 끼우면 정렬이 깨진다.
			// 검증이 gofmt 로 막기 전에 여기서 맞춘다.
			if strings.HasSuffix(rel, ".go") {
				exec.Command("gofmt", "-w", path).Run()
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

// 계획은 색인용 사본에서 세우고, 적용은 origin/main 에서 새로 딴 작업공간에서
// 한다. 사본이 뒤처져 있으면 줄 번호가 어긋나 엉뚱한 자리에 들어간다.
// 기준 줄이 제자리에 있는지 보고, 옮겨졌으면 그만큼 민다.
//
// 같은 규칙이 CIE 에도 있다. 한쪽만 고치면 조용히 어긋난다.

// anchorShift 는 기준 줄이 계획 때와 몇 줄 어긋났는지 준다.
// 찾을 수 없거나 여러 곳에 있으면 오류다 — 어림짐작으로 넣지 않는다.
func anchorShift(lines []string, c insightclient.VariantChange) (int, error) {
	if c.AnchorLine <= 0 || c.Anchor == "" {
		return 0, nil
	}
	want := strings.TrimSpace(c.Anchor)
	if i := c.AnchorLine - 1; i >= 0 && i < len(lines) && strings.TrimSpace(lines[i]) == want {
		return 0, nil
	}
	found := -1
	for i, l := range lines {
		if strings.TrimSpace(l) != want {
			continue
		}
		if found >= 0 {
			return 0, fmt.Errorf("기준 줄 %q 이 여러 곳에 있다 — 어디에 넣을지 모른다", want)
		}
		found = i
	}
	if found < 0 {
		return 0, fmt.Errorf("기준 줄 %q 을 못 찾았다 — 사본이 뒤처졌거나 코드가 바뀌었다", want)
	}
	return found - (c.AnchorLine - 1), nil
}
