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
	// 원래부터 gofmt 를 안 지키던 파일. 우리가 만든 어긋남이 아니므로
	// 다시 정렬하지도, 검증으로 막지도 않는다.
	Unformatted []string
	// 넣으면 문법이 깨져서 되돌린 자리. 사람이 봐야 한다.
	Refused []RefusedChange
}

// RefusedChange 는 넣으려다 되돌린 자리다.
type RefusedChange struct {
	File string
	Line int
	Why  string
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
		wasClean := goFileIsFormatted(path, rel)
		// 원래 파싱이 안 되던 파일은 우리 탓이 아니다. 그런 파일에 파서를 걸면
		// 멀쩡한 자리까지 되돌린다. 괄호 균형은 그래도 본다 — 그것은 넣기
		// 전후의 차이라서 원래 상태와 무관하다.
		parsedBefore := fileParses(path, rel)

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

			// 자리마다 넣어 보고 문법이 깨지면 그 자리만 되돌린다.
			//
			// 예전에는 파일을 다 고친 뒤에 한 번 검사해서, 삼항 연산자 한 자리
			// 때문에 gig_mobile 의 멀쩡한 13곳까지 통째로 날아갔다.
			// 깨진 자리만 빼고 나머지를 살린다.
			next := append(lines[:at:at], append(append([]string{}, c.Block...), lines[at:]...)...)
			if msg := lineBreaksSyntax(path, rel, string(b), next, parsedBefore); msg != "" {
				out.Refused = append(out.Refused, RefusedChange{
					File: rel, Line: at, Why: msg,
				})
				continue
			}
			lines = next
			out.Inserted++
			changed = true
		}
		if changed {
			if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
				return out, fmt.Errorf("%s 를 못 썼다: %w", rel, err)
			}
			// 정렬된 var·const 묶음에 한 줄을 끼우면 정렬이 깨진다.
			// 검증이 gofmt 로 막기 전에 여기서 맞춘다.
			//
			// 다만 원래 어긋나 있던 파일은 건드리지 않는다. gofmt 를 돌리면
			// 우리와 무관한 줄까지 다시 정렬돼 PR 에 딸려 들어간다.
			// 그런 파일은 검증에서도 뺀다 — 우리가 만든 어긋남이 아니다.
			if strings.HasSuffix(rel, ".go") {
				if wasClean {
					exec.Command("gofmt", "-w", path).Run()
				} else {
					out.Unformatted = append(out.Unformatted, rel)
				}
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

// 괄호 균형은 파일마다 원래 값이 있다 — 문자열이나 주석 안의 괄호 때문이다.
// 그 값이 넣기 전후로 같아야 한다. 0 이 아니어도 상관없다.

// balanceShift 는 넣기 전후의 괄호 균형이 달라졌으면 그 까닭을 준다.
func balanceShift(before, after string) string {
	for _, pair := range []struct{ open, close byte }{{'{', '}'}, {'(', ')'}, {'[', ']'}} {
		b := strings.Count(before, string(pair.open)) - strings.Count(before, string(pair.close))
		a := strings.Count(after, string(pair.open)) - strings.Count(after, string(pair.close))
		if a != b {
			return fmt.Sprintf("%c%c 균형이 %d 에서 %d 로 바뀌었다 — 넣은 조각이 온전하지 않다",
				pair.open, pair.close, b, a)
		}
	}
	return ""
}

// goFileIsFormatted 는 그 Go 파일이 이미 gofmt 를 지키는지 본다.
// Go 파일이 아니거나 gofmt 가 없으면 true 로 본다 — 손대지 않는 쪽이다.
func goFileIsFormatted(path, rel string) bool {
	if !strings.HasSuffix(rel, ".go") {
		return true
	}
	if _, err := exec.LookPath("gofmt"); err != nil {
		return true
	}
	b, _ := exec.Command("gofmt", "-l", path).CombinedOutput()
	return len(strings.TrimSpace(string(b))) == 0
}

// lineBreaksSyntax 는 그 자리를 넣으면 문법이 깨지는지 본다.
// 깨지지 않으면 빈 문자열이다.
//
// 괄호 균형은 언어를 몰라도 듣는다. 그 위에 파서가 있는 언어는 파서로도 본다.
func lineBreaksSyntax(path, rel, before string, after []string, parsedBefore bool) string {
	text := strings.Join(after, "\n")
	if msg := balanceShift(before, text); msg != "" {
		return msg
	}
	if !parsedBefore {
		return ""
	}
	tmp := path + ".variant-probe"
	if err := os.WriteFile(tmp, []byte(text), 0o644); err != nil {
		return ""
	}
	defer os.Remove(tmp)

	switch filepath.Ext(rel) {
	case ".go":
		cmd := exec.Command("gofmt", "-e", "-l", tmp)
		b, _ := cmd.CombinedOutput()
		if strings.Contains(string(b), tmp) && strings.Contains(string(b), ":") {
			return "Go 로 읽히지 않는다: " + firstLineOf(string(b))
		}
	case ".dart":
		if !dartParsesFile(tmp) {
			return "Dart 로 읽히지 않는다"
		}
	}
	return ""
}

// fileParses 는 그 파일이 지금 문법에 맞는지 본다.
// 파서가 없는 언어는 판단하지 않는다(true).
func fileParses(path, rel string) bool {
	switch filepath.Ext(rel) {
	case ".go":
		b, _ := exec.Command("gofmt", "-e", "-l", path).CombinedOutput()
		return !(strings.Contains(string(b), path) && strings.Contains(string(b), ":"))
	case ".dart":
		return dartParsesFile(path)
	}
	return true
}
