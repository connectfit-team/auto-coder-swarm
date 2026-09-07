package orchestrator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Dart 파일은 지금까지 한 번도 파싱된 적이 없었다.
//
// 예제 ①의 자리 23곳 가운데 18곳이 Dart 였는데, 괄호 균형만 보고 있었다.
// `dart format` 은 의존성 없이 파싱만 하므로 pub get 없이 쓸 수 있다.
//
// 함정 둘:
//   - 멀쩡한 파일도 포맷이 다르면 "Changed" 라고 한다. 문법 오류가 아니다.
//   - 파싱에 실패해도 종료 코드가 0 이다. gofmt 와 같은 함정이다.
//
// 그래서 종료 코드나 "Changed" 가 아니라 이 글귀로 판단한다.
const dartParseFailure = "could not be parsed"

// dartBin 은 Dart 실행 파일을 찾는다. 없으면 빈 문자열이다.
func dartBin() string {
	if p := strings.TrimSpace(os.Getenv("DART_BIN")); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if p, err := exec.LookPath("dart"); err == nil {
		return p
	}
	if home, err := os.UserHomeDir(); err == nil {
		p := filepath.Join(home, "tools", "dart-sdk", "bin", "dart")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// dartParses 는 그 파일이 Dart 로 읽히는지 본다.
// 실행 파일이 없으면 판단하지 않는다(true) — 확인 못 한 것은 따로 알린다.
func dartParses(dir, rel string) bool {
	bin := dartBin()
	if bin == "" {
		return true
	}
	cmd := exec.Command(bin, "format", "--output=none", rel)
	cmd.Dir = dir
	b, _ := cmd.CombinedOutput()
	return !strings.Contains(string(b), dartParseFailure)
}

// dartParsesFile 은 절대 경로 하나를 본다. 실행 파일이 없으면 판단하지 않는다.
func dartParsesFile(path string) bool {
	bin := dartBin()
	if bin == "" {
		return true
	}
	b, _ := exec.Command(bin, "format", "--output=none", path).CombinedOutput()
	return !strings.Contains(string(b), dartParseFailure)
}
