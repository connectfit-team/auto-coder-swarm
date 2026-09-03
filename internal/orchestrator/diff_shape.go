package orchestrator

import (
	"path/filepath"
	"strings"
)

// isCosmeticDiff 는 내용이 실제로 바뀌었는지 본다.
//
// 더한 줄과 뺀 줄에서 공백을 지운 것이 서로 같으면 바뀐 것이 없다. 줄 끝
// 개행, 들여쓰기, 줄바꿈만 바뀐 diff 가 여기에 걸린다.
func isCosmeticDiff(diff string) bool {
	added := map[string]int{}
	removed := map[string]int{}
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"):
			continue
		case strings.HasPrefix(line, "+"):
			if t := strings.Join(strings.Fields(line[1:]), " "); t != "" {
				added[t]++
			}
		case strings.HasPrefix(line, "-"):
			if t := strings.Join(strings.Fields(line[1:]), " "); t != "" {
				removed[t]++
			}
		}
	}
	if len(added) != len(removed) {
		return false
	}
	for k, n := range added {
		if removed[k] != n {
			return false
		}
	}
	return true
}

// 주석 한 줄의 시작 표시. 언어를 가리지 않고 흔한 것만 본다.
var commentPrefixes = []string{"//", "/*", "*/", "*", "#", "<!--", "-->", "--"}

func isCommentLine(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return true
	}
	for _, p := range commentPrefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// isCommentOnlyDiff 는 바뀐 줄이 전부 주석인지 본다.
func isCommentOnlyDiff(diff string) bool {
	changed := 0
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") {
			continue
		}
		if !strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "-") {
			continue
		}
		body := line[1:]
		if strings.TrimSpace(body) == "" {
			continue
		}
		changed++
		if !isCommentLine(body) {
			return false
		}
	}
	return changed > 0
}

// wantsCommentWork 는 요청이 주석·문서 작업인지 본다.
func wantsCommentWork(req string) bool {
	low := strings.ToLower(req)
	for _, w := range []string{"주석", "comment", "문서", "docs", "doc comment", "godoc", "jsdoc"} {
		if strings.Contains(low, w) {
			return true
		}
	}
	return false
}

// sameFilePath 는 앞의 ./ 나 / 차이를 무시하고 견준다.
func sameFilePath(a, b string) bool {
	return strings.Trim(filepath.Clean(a), "/") == strings.Trim(filepath.Clean(b), "/")
}
