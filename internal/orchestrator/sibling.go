package orchestrator

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// 남의 도구 쓰는 법은 **이웃 파일이 알고 있다.**
//
// 마지막까지 남은 잘못이 시험 파일이었다(W-69942).
//
//	Cannot find name 'page'
//	Property 'toContain' does not exist on type 'MakeMatchers<void, Locator, {}>'
//
// Playwright 는 `test('…', async ({ page }) => {…})` 로 page 를 받고,
// Locator 에는 toContain 이 아니라 toContainText 를 쓴다. 저장소 이름 쪽지는
// 이것을 알려 줄 수 없다 — 우리 모듈이 아니라 남의 도구다.
//
// 그런데 **이 저장소는 이미 그것을 바르게 쓰고 있다.** 옆에 있는 시험 파일
// 하나를 보여 주면 본떠 쓸 수 있다. 지어내는 것보다 베끼는 것이 늘 낫다.

var testFileSuffix = []string{".spec.ts", ".spec.js", ".test.ts", ".test.js", "_test.go", "_test.dart"}

func isTestFileName(p string) bool {
	low := strings.ToLower(p)
	for _, s := range testFileSuffix {
		if strings.HasSuffix(low, s) {
			return true
		}
	}
	return false
}

// siblingExample 은 같은 종류의 이웃 파일 첫머리를 준다.
//
// 같은 폴더를 먼저 보고, 없으면 위로 한 단계씩 올라간다. 고칠 파일 자신은
// 뺀다 — 새로 만드는 중이면 비어 있고, 고치는 중이면 이미 보고 있다.
func siblingExample(repoPath, file string, maxLines int) string {
	if !isTestFileName(file) {
		return ""
	}
	ext := filepath.Ext(file)
	dir := filepath.Dir(file)
	for up := 0; up < 3 && dir != "." && dir != "/"; up++ {
		if best := pickSibling(repoPath, dir, file, ext); best != "" {
			b, err := os.ReadFile(filepath.Join(repoPath, best))
			if err == nil {
				lines := strings.Split(string(b), "\n")
				if len(lines) > maxLines {
					lines = lines[:maxLines]
				}
				return "  이 저장소가 같은 종류의 시험을 쓰는 법 (" + best + ") — 본떠 써라\n" +
					"    " + strings.Join(lines, "\n    ") + "\n"
			}
		}
		dir = filepath.Dir(dir)
	}
	return ""
}

func pickSibling(repoPath, dir, self, ext string) string {
	es, err := os.ReadDir(filepath.Join(repoPath, dir))
	if err != nil {
		return ""
	}
	var cands []string
	for _, e := range es {
		if e.IsDir() {
			continue
		}
		rel := filepath.ToSlash(filepath.Join(dir, e.Name()))
		if rel == filepath.ToSlash(self) || !isTestFileName(rel) || filepath.Ext(rel) != ext {
			continue
		}
		if st, err := e.Info(); err == nil && st.Size() > 0 {
			cands = append(cands, rel)
		}
	}
	if len(cands) == 0 {
		return ""
	}
	// 이름이 가장 비슷한 것을 고른다. 같으면 짧은 쪽.
	base := strings.ToLower(filepath.Base(self))
	sort.SliceStable(cands, func(i, j int) bool {
		si, sj := sharedPrefix(base, strings.ToLower(filepath.Base(cands[i]))), sharedPrefix(base, strings.ToLower(filepath.Base(cands[j])))
		if si != sj {
			return si > sj
		}
		return len(cands[i]) < len(cands[j])
	})
	return cands[0]
}

func sharedPrefix(a, b string) int {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	return n
}
