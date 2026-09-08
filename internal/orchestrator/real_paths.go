package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
)

// 전략이 짚은 경로가 실제로 있는지 본다.
//
// 전략 단계가 "추가 검색" · "팀원과의 커뮤니케이션" · "PR 제안" 같은 **사람이
// 할 일**을 적고도 가능=true 를 냈고, 짚은 파일 셋 가운데 둘은 다른 저장소의
// 것이었다(W-82668). 관문은 actionable_path 와 total_files 가 **둘 다** 비었을
// 때만 막았으므로 그대로 지나갔다.
//
// 여기서 걸러 두면 계획 단계가 없는 경로에 새 파일을 만들지 않는다.
// 새 파일을 만드는 것 자체는 막지 않는다 — 폴더가 있으면 살린다.

// realPaths 는 그 저장소에 실제로 있는 경로만 남긴다.
//
// 전략은 경로를 저장소 이름까지 붙여 적기도 하고(ceo/internal_v2/...) 저장소
// 안의 상대 경로로 적기도 한다. 둘 다 본다.
func realPaths(repoRoot, repo string, paths []string) (kept, dropped []string) {
	for _, p := range paths {
		rel := strings.TrimSpace(p)
		if rel == "" || strings.HasPrefix(rel, "/") || strings.Contains(rel, "..") {
			dropped = append(dropped, p)
			continue
		}
		cands := []string{rel}
		if r, ok := strings.CutPrefix(rel, repo+"/"); ok {
			cands = append(cands, r)
		}
		found := ""
		for _, c := range cands {
			full := filepath.Join(repoRoot, c)
			if _, err := os.Stat(full); err == nil {
				found = c
				break
			}
			if fi, err := os.Stat(filepath.Dir(full)); err == nil && fi.IsDir() {
				found = c
				break
			}
		}
		if found == "" {
			dropped = append(dropped, p)
			continue
		}
		kept = append(kept, found)
	}
	return kept, dropped
}
