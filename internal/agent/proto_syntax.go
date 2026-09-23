package agent

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// 계약(.proto)은 쓰기 전에 문법을 본다.
//
// protoc 는 돌리지 않는다(펴내기는 make push-*apis 만 쓴다). 그래도 통째로
// 다시 쓰다 잘리거나 괄호가 어긋난 것은 그 자리에서 보인다 — 빌드는 계약을
// 안 보므로 여기서 놓치면 아무도 못 막는다. hermes 도 편집 도구 안에서
// 하위 프로세스 없이 문법을 본다(file_operations_lint.py).
//
// 확실히 틀린 것만 짚는다. 애매한 것을 막으면 멀쩡한 수정이 되돌려진다.
var (
	reProtoBlockOpen = regexp.MustCompile(`^\s*(message|enum|service|oneof|extend)\s+([A-Za-z_]\w*)?`)
	// 필드는 타입과 이름이 함께 있다. enum 값(NAME = 0;)과 갈라야 한다 —
	// enum 의 0 은 오히려 있어야 하는 것이다.
	reProtoFieldLine = regexp.MustCompile(`^(?:repeated\s+|optional\s+|required\s+)?[\w.]+\s+\w+\s*=\s*(\d+)\s*[;\[]`)
)

// checkProtoSyntax 는 계약 파일의 짜임이 깨졌는지 본다.
func checkProtoSyntax(path, content string) error {
	lines := strings.Split(content, "\n")
	depth := 0
	inBlockComment := false
	openedAt := []int{}

	for i, raw := range lines {
		line := raw
		if inBlockComment {
			if k := strings.Index(line, "*/"); k >= 0 {
				inBlockComment = false
				line = line[k+2:]
			} else {
				continue
			}
		}
		if k := strings.Index(line, "//"); k >= 0 {
			line = line[:k]
		}
		if k := strings.Index(line, "/*"); k >= 0 {
			if e := strings.Index(line[k:], "*/"); e >= 0 {
				line = line[:k] + line[k+e+2:]
			} else {
				inBlockComment = true
				line = line[:k]
			}
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// 블록을 여는 줄은 이름과 여는 괄호가 있어야 한다.
		if m := reProtoBlockOpen.FindStringSubmatch(trimmed); m != nil && !strings.HasPrefix(trimmed, "}") {
			if m[2] == "" {
				return &ParseError{Path: path, Msg: fmt.Sprintf("%d줄: %s 에 이름이 없다", i+1, m[1])}
			}
			if !strings.Contains(trimmed, "{") {
				return &ParseError{Path: path, Msg: fmt.Sprintf("%d줄: %s %s 뒤에 { 가 없다", i+1, m[1], m[2])}
			}
		}

		// 필드 번호가 쓸 수 없는 값이면 짚는다.
		if fm := reProtoFieldLine.FindStringSubmatch(trimmed); fm != nil {
			n, err := strconv.Atoi(fm[1])
			switch {
			case err != nil:
			case n == 0:
				return &ParseError{Path: path, Msg: fmt.Sprintf("%d줄: 필드 번호 0 은 쓸 수 없다", i+1)}
			case n >= 19000 && n <= 19999:
				return &ParseError{Path: path, Msg: fmt.Sprintf("%d줄: 필드 번호 %d 는 proto 가 쓰는 자리다(19000~19999)", i+1, n)}
			}
		}

		for _, c := range trimmed {
			switch c {
			case '{':
				depth++
				openedAt = append(openedAt, i+1)
			case '}':
				depth--
				if depth < 0 {
					return &ParseError{Path: path, Msg: fmt.Sprintf("%d줄: 열지 않은 } 가 있다", i+1)}
				}
				openedAt = openedAt[:len(openedAt)-1]
			}
		}
	}

	if inBlockComment {
		return &ParseError{Path: path, Msg: "주석이 닫히지 않았다 — 파일이 잘린 것 같다"}
	}
	if depth > 0 {
		return &ParseError{Path: path, Msg: fmt.Sprintf("%d줄에서 연 { 가 닫히지 않았다 — 파일이 잘린 것 같다", openedAt[0])}
	}
	return nil
}
