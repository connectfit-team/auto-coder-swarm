package agent

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// 고를 것과 쓸 것을 나눈다.
//
// 찾아바꾸기는 한 번에 셋을 시킨다 — 자리를 찾고, 코드를 쓰고, 형식을 맞춘다.
// 9B 는 셋 다 어설프게 하고, 실패는 늘 첫째에서 났다.
//
//	원문에 없는 내용을 찾으라고 했다: await expect(body).not.toContainText('
//	import { t } from '…'  +  const { t } = useI18n();   ← 떨어진 줄을 이어 붙였다
//	Identifier 'pending' has already been declared        ← 같은 것을 또 넣었다
//
// 나누면 그 갈래가 통째로 사라진다.
//
//	하나. 「어느 줄 뒤에 넣나? 번호만 답해라」      ← 선택 하나
//	둘.  「그 자리에 들어갈 코드만 써라」           ← 쓰기 하나
//	셋.  붙이는 것은 기계가 한다                   ← 틀릴 데가 없다

type editSpot struct {
	line int    // 1부터 센다
	mode string // after · before · replace
}

var spotRe = regexp.MustCompile(`(?i)(\d+)\s*(?:줄)?\s*(뒤|앞|고침|after|before|replace)?`)

// parseSpot 은 모델의 짧은 답에서 줄 번호와 무엇을 할지를 읽는다.
func parseSpot(raw string, maxLine int) (editSpot, error) {
	m := spotRe.FindStringSubmatch(strings.TrimSpace(raw))
	if m == nil {
		return editSpot{}, fmt.Errorf("줄 번호를 못 읽었다: %q", clipRunes(raw, 120))
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n < 1 || n > maxLine {
		return editSpot{}, fmt.Errorf("줄 번호가 파일 밖이다(1~%d): %q", maxLine, clipRunes(raw, 120))
	}
	mode := "after"
	switch strings.ToLower(m[2]) {
	case "앞", "before":
		mode = "before"
	case "고침", "replace":
		mode = "replace"
	}
	return editSpot{line: n, mode: mode}, nil
}

// spliceSnippet 은 고른 자리에 코드를 넣는다. 들여쓰기는 그 자리 것을 물려준다.
func spliceSnippet(src string, spot editSpot, snippet string) string {
	lines := strings.Split(src, "\n")
	if spot.line < 1 || spot.line > len(lines) {
		return src
	}
	anchor := lines[spot.line-1]
	body := reindent(strings.Split(strings.TrimRight(snippet, "\n"), "\n"), leadingSpace(anchor))

	out := make([]string, 0, len(lines)+len(body))
	switch spot.mode {
	case "before":
		out = append(out, lines[:spot.line-1]...)
		out = append(out, body...)
		out = append(out, lines[spot.line-1:]...)
	case "replace":
		out = append(out, lines[:spot.line-1]...)
		out = append(out, body...)
		out = append(out, lines[spot.line:]...)
	default: // after
		out = append(out, lines[:spot.line]...)
		out = append(out, body...)
		out = append(out, lines[spot.line:]...)
	}
	return strings.Join(out, "\n")
}

// numberedAll 은 파일 전체에 줄 번호를 붙인다. 자리를 고르려면 번호가 있어야 한다.
func numberedAll(src string, max int) (string, int) {
	lines := strings.Split(src, "\n")
	n := len(lines)
	if n > max {
		lines = lines[:max]
	}
	var b strings.Builder
	for i, l := range lines {
		fmt.Fprintf(&b, "%d: %s\n", i+1, l)
	}
	return b.String(), n
}
