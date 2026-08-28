package orchestrator

import (
	"context"
	"os"
	"os/exec"
	"strings"
)

// 빌드·검증 명령을 만든다.
//
// **systemd 는 로그인 셸의 PATH 를 물려주지 않는다.** 그래서 `go build ./...`
// 이 `go: command not found` 로 죽었다 — ACS 는 Go 빌드를 **한 번도 검증한
// 적이 없다.** 세 번 자가치유를 돌리고 세 번 다시 계획을 세운 뒤 실패했는데,
// 원인은 늘 같은 한 줄이었다.
//
// 유닛에 PATH 를 박아 넣어도 되지만, 그러면 배포하는 사람이 그걸 알아야 한다.
// 프로세스가 스스로 챙기는 쪽이 낫다.
//
// 여기 없는 도구를 쓰게 되면 그때 더한다. 없는 경로는 그냥 무시된다.
var toolchainDirs = []string{
	"/usr/local/go/bin",        // go
	"/usr/local/bin",           // rg, gh 등
	"/home/cnf/go/bin",         // go install 로 받은 것
	"/home/cnf/.nvm/versions",  // node (버전 디렉터리는 아래에서 펼친다)
	"/home/cnf/.pub-cache/bin", // dart/flutter
	"/opt/flutter/bin",
}

// shellCmd 는 저장소 안에서 셸 한 줄을 돌린다. PATH 에 도구 경로를 얹는다.
func shellCmd(ctx context.Context, dir, script string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "bash", "-c", script)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(), "PATH="+augmentedPath())
	return cmd
}

// augmentedPath 는 이미 있는 PATH 앞에 도구 경로를 붙인다.
// 중복은 넣지 않는다 — PATH 가 길어지면 어디서 온 도구인지 읽기 어려워진다.
func augmentedPath() string {
	cur := os.Getenv("PATH")
	have := map[string]bool{}
	for _, p := range strings.Split(cur, ":") {
		have[p] = true
	}

	var add []string
	for _, d := range toolchainDirs {
		if have[d] {
			continue
		}
		if st, err := os.Stat(d); err != nil || !st.IsDir() {
			continue
		}
		have[d] = true
		add = append(add, d)
	}
	if len(add) == 0 {
		return cur
	}
	return strings.Join(add, ":") + ":" + cur
}
