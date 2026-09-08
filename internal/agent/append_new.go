package agent

import (
	"fmt"
	"regexp"
	"strings"
)

// 새 기능은 **찾아 바꾸는 것이 아니라 붙이는 것**이다.
//
// 코더는 "이 내용을 찾아 바꿔라" 하나로만 일했다. 그래서 없는 기능을 만들
// 때마다 이렇게 죽었다(W-57730).
//
//	원문에 없는 내용을 찾으라고 했다:
//	// CEOWorkConnectionUpdate 메서드를 구현합니다.
//
// 당연하다. 새로 만드는 메서드는 원문에 없다. 찾을 것이 아니라 **형제 옆에
// 붙일 것**이다. 어디에 붙일지는 이미 알 수 있다 — 가장 비슷한 곳을 짚는
// 기계가 있고, 실측으로 낱말 5/5 로 제 형제를 정확히 짚었다.
//
//	원문에서 가장 비슷한 곳은 160줄이다 (낱말 5/5 겹침):
//	164: func (s *ConnectService) ConnectedWorkplaceIDListGet(ctx …
//
// 함부로 붙이면 안 되므로 조건을 둔다.
//   - 넣을 것이 **온전한 선언**이어야 한다(func 또는 인터페이스 메서드 한 줄).
//     조각을 붙이면 문법이 깨진다.
//   - 형제와 **충분히 겹쳐야** 한다. 엉뚱한 곳에 붙이는 것은 실패보다 나쁘다.
//   - 형제 선언이 **끝나는 자리** 뒤에 붙인다. 함수 안에 함수를 넣으면 안 된다.
//   - 이미 그 이름이 있으면 붙이지 않는다 — 두 번 선언된다.

var (
	funcDeclRe       = regexp.MustCompile(`^\s*func\s+(\([^)]*\)\s*)?([A-Za-z_]\w*)\s*\(`)
	ifaceMethodRe    = regexp.MustCompile(`^\s*([A-Z]\w*)\s*\([^)]*\)\s*(\(?[\w\[\]\*\., ]*\)?)?\s*$`)
	appendMinOverlap = 2
)

// declaredName 은 넣을 것이 무엇을 선언하는지 준다.
// 온전한 선언이 아니면 빈 문자열이다.
func declaredName(block string) string {
	lines := strings.Split(strings.TrimSpace(block), "\n")
	// 앞의 주석은 선언의 일부다.
	i := 0
	for i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), "//") {
		i++
	}
	if i >= len(lines) {
		return "" // 주석만 있는 것은 선언이 아니다
	}
	head := lines[i]
	if m := funcDeclRe.FindStringSubmatch(head); m != nil {
		// 여러 줄 함수는 중괄호가 닫혀야 온전하다.
		if strings.Count(block, "{") > 0 && strings.Count(block, "{") == strings.Count(block, "}") {
			return m[2]
		}
		return ""
	}
	if len(lines)-i == 1 {
		if m := ifaceMethodRe.FindStringSubmatch(head); m != nil {
			return m[1]
		}
	}
	return ""
}

// appendBesideSibling 은 넣을 선언을 가장 비슷한 형제 뒤에 붙인다.
//
// 붙일 수 없으면 (왜인지, false)를 준다.
func appendBesideSibling(srcLines []string, search, replace string) (string, string, bool) {
	name := declaredName(replace)
	if name == "" {
		return "", "넣을 것이 온전한 선언이 아니다 — 조각은 붙일 수 없다", false
	}
	// 이미 있으면 붙이면 안 된다. 두 번 선언된다.
	for _, l := range srcLines {
		if m := funcDeclRe.FindStringSubmatch(l); m != nil && m[2] == name {
			return "", name + " 는 이미 있다", false
		}
	}

	at, n, hit, want := bestSibling(srcLines, search+"\n"+replace)
	if at < 0 || hit < appendMinOverlap {
		return "", fmt.Sprintf("붙일 형제를 못 찾았다 (겹침 %d/%d)", hit, want), false
	}

	end := declEnd(srcLines, at, n)
	out := make([]string, 0, len(srcLines)+8)
	out = append(out, srcLines[:end+1]...)
	out = append(out, "")
	out = append(out, reindentTo(srcLines[at], strings.Split(strings.TrimRight(replace, "\n"), "\n"))...)
	out = append(out, srcLines[end+1:]...)
	return strings.Join(out, "\n"),
		fmt.Sprintf("%s 를 %d줄의 형제 뒤에 붙였다 (겹침 %d/%d)", name, at+1, hit, want), true
}

// declEnd 는 형제 선언이 끝나는 줄이다. 함수라면 중괄호가 닫히는 줄까지.
func declEnd(srcLines []string, at, n int) int {
	depth := 0
	started := false
	for i := at; i < len(srcLines); i++ {
		depth += strings.Count(srcLines[i], "{") - strings.Count(srcLines[i], "}")
		if strings.Contains(srcLines[i], "{") {
			started = true
		}
		if started && depth <= 0 {
			return i
		}
		if !started && i >= at+n-1 {
			return i // 인터페이스 메서드처럼 몸통이 없는 것
		}
	}
	return min(len(srcLines)-1, at+n-1)
}

// reindentTo 는 붙일 줄들을 형제의 들여쓰기에 맞춘다.
func reindentTo(sibling string, block []string) []string {
	pad := sibling[:len(sibling)-len(strings.TrimLeft(sibling, " \t"))]
	if pad == "" {
		return block
	}
	base := ""
	for _, l := range block {
		if strings.TrimSpace(l) != "" {
			base = l[:len(l)-len(strings.TrimLeft(l, " \t"))]
			break
		}
	}
	out := make([]string, len(block))
	for i, l := range block {
		out[i] = pad + strings.TrimPrefix(l, base)
	}
	return out
}
