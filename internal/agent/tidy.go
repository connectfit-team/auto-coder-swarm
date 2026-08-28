package agent

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// 모델이 쓴 파일을 기계가 다듬는다.
//
// 9B 가 쓴 Go 는 import 를 자주 틀린다. 실측으로 자가치유를 세 번 돌리고도
// 못 고친 오류가 이것뿐이었다:
//
//	internal/domain/tag.go:7:2: "github.com/jinzhu/gorm" imported and not used
//	internal/domain/tag.go:28:14: undefined: strconv
//
// **둘 다 goimports 가 0초에 고친다.** 모델에게 세 번 더 물어볼 일이 아니다.
// 도구가 없으면 조용히 넘어간다 — 다듬기는 있으면 좋은 것이지 필수가 아니다.
func TidyFile(ctx context.Context, path string) {
	if !strings.HasSuffix(path, ".go") {
		return
	}
	bin := findTool("goimports")
	if bin == "" {
		bin = findTool("gofmt") // 최소한 형식이라도 맞춘다
	}
	if bin == "" {
		return
	}
	c, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	exec.CommandContext(c, bin, "-w", path).Run()
}

// findTool 은 PATH 와 흔한 설치 위치에서 도구를 찾는다.
// systemd 는 로그인 셸의 PATH 를 안 물려주므로 PATH 만 믿을 수 없다.
func findTool(name string) string {
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	for _, d := range []string{"/home/cnf/go/bin", "/usr/local/go/bin", "/usr/local/bin", "/usr/bin"} {
		p := filepath.Join(d, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}
