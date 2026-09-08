package agent

import (
	"fmt"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
)

// 통짜로 다시 쓴 파일이 문법에 맞는지 본다.
//
// 작은 파일은 모델에게 통째로 다시 받는다. 그러면 모델이 조용히 망가뜨린다 —
// 실측으로 이런 것이 그대로 저장됐다(W-77855).
//
//	internal_v2/mariadb/connect.go:120:3: non-declaration statement outside function body
//	internal_v2/mariadb/connect.go:137:2: unexpected name cordNotFound after top level declaration
//
// `cordNotFound` 는 `RecordNotFound` 의 앞이 잘려 나간 것이다. 사라진 export 는
// 막고 있었지만(lostExports) **글자가 뭉개진 것**은 아무도 안 봤다. 그대로
// 저장되어 빌드가 깨지고, 자가치유가 그 위에서 또 고치려 든다.
//
// 파싱은 빌드보다 싸고 즉시다. 쓰기 전에 본다.

// ParseError 는 문법이 깨졌다는 것과 그 자리다.
type ParseError struct {
	Path string
	Msg  string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("%s: 다시 쓴 내용이 문법에 안 맞는다 — %s", filepath.Base(e.Path), e.Msg)
}

// CheckSyntax 는 그 내용이 그 언어의 문법에 맞는지 본다.
// 검사할 수 없는 언어는 통과시킨다 — 없는 검사를 실패로 만들면 안 된다.
func CheckSyntax(path, content string) error {
	if strings.TrimSpace(content) == "" {
		return &ParseError{Path: path, Msg: "빈 내용이다"}
	}
	if !strings.HasSuffix(path, ".go") {
		return nil
	}
	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, path, content, parser.AllErrors); err != nil {
		return &ParseError{Path: path, Msg: firstParseLine(err.Error())}
	}
	return nil
}

// firstParseLine 은 파서 오류의 첫 줄만 준다. 스무 줄을 되먹임에 실으면
// 정작 첫 줄이 잘려 나간다.
func firstParseLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	out := lines[0]
	if len(lines) > 1 {
		out += fmt.Sprintf(" (그 밖에 %d줄)", len(lines)-1)
	}
	return out
}
