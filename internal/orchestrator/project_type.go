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
	// 기준선이 이미 깨져 있을 때 물러설 곳. 엄한 것부터 적는다.
	//
	// 저장소에 원래 깨져 있는 시험 파일이 있으면 엄한 명령은 손대기 전부터
	// 실패한다. 그대로 두면 그 저장소는 **영원히 작업할 수 없다** — 실측으로
	// ceo 의 connect_handler_disconnect_test.go 가 옛 시그니처를 부르고 있어
	// "연결보류 기능 추가" 요청이 계획 단계에서 죽었다(W-80655). 그 시험은
	// 이 요청과 아무 상관이 없다.
	weaker []string
}

// 위에서부터 먼저 맞는 것을 쓴다. 여러 개가 섞인 저장소는 앞의 것이 이긴다
// (예: Flutter 저장소 안의 node 도구).
var projectDefaults = []projectDefault{
	{"pubspec.yaml", "Flutter", "flutter analyze", nil},
	// **`go build` 는 _test.go 를 통째로 건너뛴다.** ACS 가 쓴 테스트 파일이
	// 컴파일조차 안 되는데 "빌드 성공" 으로 지나갔다(실증). `go test` 로
	// 컴파일까지 시키되 테스트는 돌리지 않는다(`-run ^$`) — 실행은 아래
	// 단계에서 바뀐 패키지만 한다.
	{"go.mod", "Go", "go build ./... && go test -run '^$' -count=1 -vet=off ./...",
		// 시험 파일이 원래 깨져 있으면 소스만 본다. 우리가 쓴 시험 파일을
		// 못 잡게 되므로, 물러섰다는 것을 PR 에 적어야 한다.
		[]string{"go build ./..."}},
	{"Cargo.toml", "Rust", "cargo check", nil},
	{"pom.xml", "Java", "mvn -q -DskipTests compile", nil},
	{"build.gradle", "Java", "./gradlew assemble", nil},
	{"pyproject.toml", "Python", "python -m compileall -q .", nil},
	{"requirements.txt", "Python", "python -m compileall -q .", nil},
	{"package.json", "NodeJS", "npm run build --if-present", nil},
}

// detectProjectFallback 은 표식 파일로 종류와 빌드 명령을 정한다.
// 못 찾으면 빈 값을 돌려준다 — 부르는 쪽이 그걸 보고 판단한다.
func detectProjectFallback(repoPath string) (kind, build string) {
	k, b, _ := detectProject(repoPath)
	return k, b
}

// detectProject 는 종류·엄한 명령·물러설 명령들을 준다.
func detectProject(repoPath string) (kind, build string, weaker []string) {
	for _, d := range projectDefaults {
		if _, err := os.Stat(filepath.Join(repoPath, d.marker)); err == nil {
			return d.kind, d.build, d.weaker
		}
	}
	return "", "", nil
}
