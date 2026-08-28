package orchestrator

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
)

// 바뀐 패키지의 테스트를 실제로 돌린다.
//
// 컴파일만 보면 **틀린 테스트가 통과한다.** ACS 가 테스트를 쓰는 것이 일의
// 절반인데, 그게 실제로 도는지는 돌려 봐야 안다.
//
// 저장소 전체를 돌리지 않는다 — DB·네트워크를 타는 테스트까지 걸려 오래
// 걸리고, 우리가 건드리지도 않은 곳에서 실패해 엉뚱한 자가치유가 돈다.
// **바뀐 파일이 있는 패키지만** 돌린다.
func (t *taskContext) runChangedTests(plan agent.Plan) (string, bool) {
	if t.meta.Type != "Go" {
		return "", true // 지금은 Go 만. 다른 언어는 빌드 검증까지다
	}

	dirs := changedDirs(plan)
	if len(dirs) == 0 {
		return "", true
	}

	args := make([]string, 0, len(dirs))
	for _, d := range dirs {
		args = append(args, "./"+d+"/...")
	}
	script := "go test -count=1 -vet=off " + strings.Join(args, " ")

	out, err := shellCmd(t.ctx, t.repoPath, script).CombinedOutput()
	if err == nil {
		return string(out), true
	}
	return fmt.Sprintf("%s 실패:\n%s", script, string(out)), false
}

// changedDirs 는 계획이 건드린 파일들의 디렉터리를 모은다.
func changedDirs(plan agent.Plan) []string {
	seen := map[string]bool{}
	var out []string
	for _, c := range plan.Changes {
		d := filepath.Dir(filepath.ToSlash(c.FilePath))
		// 저장소 뿌리는 `./...` 이 되어 전체를 돌린다. 그건 피한다.
		if d == "." || d == "/" || d == "" || seen[d] {
			continue
		}
		// 상위로 빠져나가는 경로는 쓰지 않는다.
		if strings.HasPrefix(d, "..") || strings.HasPrefix(d, "/") {
			continue
		}
		seen[d] = true
		out = append(out, d)
	}
	sort.Strings(out) // 같은 계획이면 같은 명령이 되게
	return out
}
