package orchestrator

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
)

// 이미 무엇이 있는지 **세어서 보여 준다.**
//
// 자식이 이렇게 죽었다 — 「SessionService 가 이미 존재하는지, 또는 다른
// 서비스에 추가해야 하는지 확인하지 못했습니다」. 모델이 모를 만하다.
// 넘길 때 파일 이름만 줬지 그 안에 무엇이 있는지는 안 줬다.
//
// aider 는 이것을 repomap 으로 푼다 — tree-sitter 로 심볼 그래프를 만들고
// pagerank 로 순위를 매겨 예산에 맞춰 지도를 준다. 그리고 모델에게 **고르라고
// 하지 않는다.** 이름을 대게 할 뿐이다.
//
// 계약은 그래프까지 갈 것도 없다. 선언을 읽어 순서만 매기면 된다 —
// 고칠 파일이 먼저, 그 다음이 같은 폴더.
var (
	reOutlineDecl = regexp.MustCompile(`(?m)^\s*(message|enum|service)\s+([A-Za-z_]\w*)`)
	reOutlineRPC  = regexp.MustCompile(`(?m)^\s*rpc\s+([A-Za-z_]\w*)`)
)

const outlineBudget = 2000

// protoOutline 은 그 폴더의 계약에 이미 있는 것을 간추려 준다.
func protoOutline(repoPath string, focus ...string) string {
	if repoPath == "" {
		return ""
	}
	want := map[string]bool{}
	dirs := map[string]bool{}
	for _, f := range focus {
		if f == "" {
			continue
		}
		want[f] = true
		dirs[path.Dir(f)] = true
	}

	type entry struct {
		rel   string
		lines []string
		first bool // 고칠 파일이다
	}
	var entries []entry
	forEachProto(repoPath, func(rel, src string) {
		if len(dirs) > 0 && !dirs[path.Dir(rel)] {
			return
		}
		var lines []string
		for _, m := range reOutlineDecl.FindAllStringSubmatch(src, -1) {
			if m[1] == "service" {
				var rpcs []string
				for _, r := range reOutlineRPC.FindAllStringSubmatch(src, -1) {
					rpcs = append(rpcs, r[1])
				}
				lines = append(lines, fmt.Sprintf("service %s { %s }", m[2], strings.Join(rpcs, " · ")))
				continue
			}
			lines = append(lines, m[1]+" "+m[2])
		}
		if len(lines) > 0 {
			entries = append(entries, entry{rel: rel, lines: lines, first: want[rel]})
		}
	})
	if len(entries) == 0 {
		return ""
	}
	// 고칠 파일이 먼저, 그 다음은 이름 차례로 — 회차마다 같은 글이 되게.
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].first != entries[j].first {
			return entries[i].first
		}
		return entries[i].rel < entries[j].rel
	})

	var b strings.Builder
	b.WriteString("[이 계약에 이미 있는 것 — 새로 만들지 말고 이 가운데서 골라 쓴다]\n")
	for _, e := range entries {
		if b.Len() > outlineBudget {
			b.WriteString("… 그 밖의 파일은 줄인다\n")
			break
		}
		b.WriteString(e.rel + "\n")
		for _, l := range e.lines {
			if b.Len() > outlineBudget {
				break
			}
			b.WriteString("  " + l + "\n")
		}
	}
	return b.String()
}
