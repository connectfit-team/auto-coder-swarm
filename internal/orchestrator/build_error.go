package orchestrator

import (
	"strings"
	"unicode/utf8"
)

// clip 은 UTF-8 경계에서 자르고 잘렸다는 것을 알린다.
func clip(s string, max int) string {
	if len(s) <= max {
		return s
	}
	s = s[:max]
	for len(s) > 0 && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s + "\n… (이하 생략)"
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// 테스트 실패를 빌드 실패와 같은 길로 보낸다 — 자가치유가 고칠 대상이다.
var errChangedTestsFailed = &changedTestsError{}

type changedTestsError struct{}

func (*changedTestsError) Error() string { return "바뀐 패키지의 테스트가 실패했다" }

// 빌드 출력에서 오류로 보이는 줄.
var buildErrorMarkers = []string{
	"error", "err!", "failed", "failure", "cannot find", "not found",
	"unexpected", "is not assignable", "does not exist", "syntaxerror",
	"typeerror", "referenceerror", "missing", "expected",
}

// 경고일 뿐인 줄. 기준 빌드도 이것을 달고 통과한다.
var buildNoiseMarkers = []string{
	"a11y:", "warning", "warn ", "deprecated", "npm notice",
	"browserslist", "vite v", "building ", "transforming",
}

// distillBuildError 는 빌드 출력에서 고칠 거리가 되는 줄만 남긴다.
//
// 앞에서 자르면 안 된다 — 빌드 도구는 경고를 먼저, 오류를 나중에 찍는다.
// 아무것도 못 고르면 앞이 아니라 **뒤**를 준다.
func distillBuildError(out string) string {
	const maxLines = 40
	lines := strings.Split(out, "\n")

	var kept []string
	for i, ln := range lines {
		low := strings.ToLower(ln)
		noisy := false
		for _, m := range buildNoiseMarkers {
			if strings.Contains(low, m) {
				noisy = true
				break
			}
		}
		if noisy {
			continue
		}
		for _, m := range buildErrorMarkers {
			if strings.Contains(low, m) {
				kept = append(kept, ln)
				// 오류 줄 바로 다음 두 줄에 파일·위치가 붙는다.
				for j := i + 1; j < len(lines) && j <= i+2; j++ {
					if strings.TrimSpace(lines[j]) != "" {
						kept = append(kept, lines[j])
					}
				}
				break
			}
		}
		if len(kept) >= maxLines {
			break
		}
	}

	if len(kept) == 0 {
		// 고를 것이 없으면 끝부분을 준다. 오류는 대개 마지막에 있다.
		tail := lines
		if len(tail) > maxLines {
			tail = tail[len(tail)-maxLines:]
		}
		return strings.TrimSpace(strings.Join(tail, "\n"))
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}

// 이만큼 지워지면 "고친" 것이 아니라 "날린" 것으로 본다.
const (
	wipeMinDeleted = 50 // 이보다 적게 지웠으면 정상적인 정리일 수 있다
	wipeRatio      = 5  // 지운 줄이 더한 줄의 이 배를 넘으면 의심한다
)
