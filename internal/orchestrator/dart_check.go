package orchestrator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Dart 는 `dart format` 으로 파싱한다 — 의존성 없이 파싱만 하므로 pub get 이
// 필요 없다.
//
// 함정 셋:
//   - 멀쩡한 파일도 포맷이 다르면 "Changed" 라고 한다. 문법 오류가 아니다.
//   - 파싱 실패는 종료 코드 65 지만, **없는 파일은 종료 코드 0** 이다.
//     경로가 틀리면 아무것도 검사하지 않고 통과가 된다.
//   - 실행 파일이 없으면 판단하지 않는다. 그때는 Unverified 로 따로 알린다.
//
// 그래서 종료 코드가 아니라 이 두 글귀로 판단한다.
const (
	dartParseFailure   = "could not be parsed"
	dartNothingChecked = "Formatted no files"
)

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
// 빈 문자열이면 통과, 그 밖은 사람이 읽을 까닭이다.
// 실행 파일이 없으면 판단하지 않는다(빈 문자열) — 확인 못 한 것은 따로 알린다.
func dartParses(path string) string {
	bin := dartBin()
	if bin == "" {
		return ""
	}
	if msg := fileMissing(path); msg != "" {
		return msg
	}
	b, _ := exec.Command(bin, "format", "--output=none", path).CombinedOutput()
	out := string(b)
	if strings.Contains(out, dartParseFailure) {
		return "Dart 로 읽히지 않는다: " + dartWhere(out)
	}
	if strings.Contains(out, dartNothingChecked) {
		// 없는 파일이 여기로 온다 — 종료 코드는 0 이다. 검사가 돌지
		// 않았다는 뜻이므로 통과로 세면 안 된다.
		return "Dart 검사가 파일을 읽지 못했다: " + path
	}
	return ""
}

// dartWhere 는 "line 2, column 1 of a.dart: ..." 줄만 뽑는다.
// 그 앞에는 요약, 뒤에는 화살표 그림이 붙어 나온다.
func dartWhere(out string) string {
	for _, l := range strings.Split(out, "\n") {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "line ") {
			return t
		}
	}
	return firstLineOf(out)
}

// fileMissing 은 검사할 파일이 실제로 있는지 본다.
//
// 없는 파일을 검사기에 주면 도구마다 다르게 실패한다 — dart format 은 종료
// 코드 0 을 주고, node 는 스택 추적을 뱉는다. 어느 쪽도 "문법이 틀렸다" 가
// 아니고 "검사가 안 돌았다" 다. 여기서 먼저 걸러 한 가지 말로 준다.
func fileMissing(path string) string {
	if _, err := os.Stat(path); err != nil {
		return "검사할 파일이 없다: " + path
	}
	return ""
}
