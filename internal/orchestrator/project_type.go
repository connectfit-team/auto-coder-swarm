package orchestrator

import (
	"os"
	"path/filepath"
)

// 프로젝트 종류는 **파일로 알 수 있다.** 모델에게 물을 일이 아니다.
//
// 종류 판별을 LLM 에 맡겼더니 빈 빌드 명령이 돌아왔고, `bash -c ""` 는 exit 0
// 이라 **아무것도 검증하지 않고 "빌드 성공" 으로 지나갔다.** 로그에는
// `[BUILD] [] 검증 ()` 만 남는다.
//
// go.mod 가 있으면 Go 다. 그건 모델의 판단이 아니라 사실이다.

type projectDefault struct {
	marker string
	kind   string
	build  string
}

// 위에서부터 먼저 맞는 것을 쓴다. 여러 개가 섞인 저장소는 앞의 것이 이긴다
// (예: Flutter 저장소 안의 node 도구).
var projectDefaults = []projectDefault{
	{"pubspec.yaml", "Flutter", "flutter analyze"},
	// **`go build` 는 _test.go 를 통째로 건너뛴다.** ACS 가 쓴 테스트 파일이
	// 컴파일조차 안 되는데 "빌드 성공" 으로 지나갔다(실증). `go test` 로
	// 컴파일까지 시키되 테스트는 돌리지 않는다(`-run ^$`) — 실행은 아래
	// 단계에서 바뀐 패키지만 한다.
	{"go.mod", "Go", "go build ./... && go test -run '^$' -count=1 -vet=off ./..."},
	{"Cargo.toml", "Rust", "cargo check"},
	{"pom.xml", "Java", "mvn -q -DskipTests compile"},
	{"build.gradle", "Java", "./gradlew assemble"},
	{"pyproject.toml", "Python", "python -m compileall -q ."},
	{"requirements.txt", "Python", "python -m compileall -q ."},
	{"package.json", "NodeJS", "npm run build --if-present"},
}

// detectProjectFallback 은 표식 파일로 종류와 빌드 명령을 정한다.
// 못 찾으면 빈 값을 돌려준다 — 부르는 쪽이 그걸 보고 판단한다.
func detectProjectFallback(repoPath string) (kind, build string) {
	for _, d := range projectDefaults {
		if _, err := os.Stat(filepath.Join(repoPath, d.marker)); err == nil {
			return d.kind, d.build
		}
	}
	return "", ""
}
