package orchestrator

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
)

// 치유기가 적은 파일 이름을 **계획에 있는 경로로 되찾는다.**
//
// 모델이 `+page.server.ts` 라고만 적었다. 그대로 이어 붙이면
// `<워크트리>/+page.server.ts` 가 되고, 그런 파일은 없다.
//
//	failed to read file: …/repo/+page.server.ts: no such file or directory
//
// 폴더가 깊은 파일은 **영원히 못 고친다.** 실제로 그 회차는 아무것도 못
// 고치고 "코드가 하나도 바뀌지 않았다" 로 끝났다(W-14440).
//
// 짧게 적은 것을 나무랄 일이 아니다. 어느 파일인지 알 수 있으면 찾아 주면
// 된다 — 계획이 짚은 파일과 빌드 오류에 나온 파일이 그 목록이다. 다만
// **하나로 좁혀질 때만** 쓴다. 둘 이상이면 어느 쪽인지 알 수 없다.

// resolveHealTarget 는 고칠 파일의 저장소 기준 경로를 준다. 못 찾으면 false.
func resolveHealTarget(repoPath, target string, plan agent.Plan, failure string) (string, bool) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", false
	}
	if st, err := os.Stat(filepath.Join(repoPath, target)); err == nil && !st.IsDir() {
		return target, true
	}

	base := filepath.Base(target)
	seen := map[string]bool{}
	var hits []string
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] || filepath.Base(p) != base {
			return
		}
		if st, err := os.Stat(filepath.Join(repoPath, p)); err != nil || st.IsDir() {
			return
		}
		seen[p] = true
		hits = append(hits, p)
	}

	for _, c := range plan.Changes {
		add(c.FilePath)
	}
	for _, p := range pathsInText(failure) {
		add(p)
	}

	if len(hits) == 1 {
		return hits[0], true
	}
	return "", false
}

// pathsInText 는 빌드 오류에 나온 저장소 기준 경로를 줍는다.
func pathsInText(s string) []string {
	var out []string
	for _, f := range strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '"' || r == '\'' || r == '(' || r == ')'
	}) {
		f = strings.TrimRight(f, ":,;")
		if i := strings.Index(f, ":"); i > 0 && strings.Contains(f[:i], "/") {
			f = f[:i] // path:line:col
		}
		if strings.Contains(f, "/") && strings.Contains(filepath.Base(f), ".") {
			out = append(out, strings.TrimPrefix(f, "./"))
		}
	}
	return out
}
