package agent

import (
	"fmt"
	"regexp"
	"strings"
)

// 원본을 통째로 다시 쓰게 해도 되는 크기. 글자 수 기준이다.
//
// 창이 8,192 토큰이다. 원본을 넣고 **다시 전부 출력**하게 하면 같은 내용이
// 두 번 들어가므로 그보다 훨씬 작아야 한다. 536줄짜리 attendance.ts 로
// 시켰더니 출력이 잘려 기존 export 가 사라졌고, 빌드가
// `"getAttendanceRecords" is not exported` 로 깨졌다.
const wholeFileRewriteLimit = 6000

// 찾아바꾸기 블록. 작은 모델이 통짜 파일보다 훨씬 잘 낸다.
const editBlockFormat = "<<<<<<< SEARCH\n(원문에 그대로 있는 줄)\n=======\n(바꿀 내용)\n>>>>>>> REPLACE"

// 형식 요구. 프롬프트 맨 끝에 붙인다 — 앞에 두면 모델이 잊는다.
const editBlockRules = `이제 고칠 자리만 내라. **설명하지 마라. 파일 전체를 내지 마라.**
답의 첫 글자는 < 여야 한다.

` + editBlockFormat + `

규칙:
1. SEARCH 에는 원문에 있는 그대로 옮겨 적어라. 줄 번호는 빼고 적어라.
2. SEARCH 는 원문에서 한 번만 나오도록 앞뒤 줄을 충분히 넣어라.
3. 블록 밖에는 아무 말도 쓰지 마라.
4. 고치라고 한 것만 고쳐라. 블록은 여러 개여도 된다.`

var editBlockRe = regexp.MustCompile(`(?s)<{5,}\s*SEARCH\s*\n(.*?)\n={5,}\s*\n(.*?)\n>{5,}\s*REPLACE`)

// applyEditBlocks 는 찾아바꾸기 블록을 원문에 적용한다.
//
// 찾는 줄이 없거나 여러 번 나오면 **적용하지 않는다.** 어디를 고치는지
// 확실하지 않은 채로 쓰면 엉뚱한 자리를 바꾼다.
func applyEditBlocks(original, raw string) (string, error) {
	ms := editBlockRe.FindAllStringSubmatch(raw, -1)
	if len(ms) == 0 {
		return "", fmt.Errorf("찾아바꾸기 블록이 없다 — 형식은 이렇다:\n%s", editBlockFormat)
	}
	out := original
	for _, m := range ms {
		search, replace := stripLineNumbers(m[1]), stripLineNumbers(m[2])
		if strings.TrimSpace(search) == "" {
			return "", fmt.Errorf("찾을 내용이 비었다")
		}
		switch strings.Count(out, search) {
		case 1:
			out = strings.Replace(out, search, replace, 1)
			continue
		case 0:
			// **들여쓰기까지 똑같이 옮겨 적기를 바랄 수는 없다.**
			//
			// 모델이 자리는 정확히 짚고도 앞 공백이 달라 못 찾는 일이 잦다.
			// 줄 끝 공백을 털고 줄 단위로 견줘 한 군데만 맞으면 그 자리를 쓴다.
			replaced, err := replaceLoosely(out, search, replace)
			if err != nil {
				return "", err
			}
			out = replaced
		default:
			return "", fmt.Errorf("원문에 여러 번 나오는 내용이라 어디인지 알 수 없다:\n%s", clipRunes(search, 200))
		}
	}
	if out == original {
		return "", fmt.Errorf("블록을 적용했지만 바뀐 것이 없다")
	}
	return out, nil
}

// 파일이 밖으로 내주던 이름. 이것이 사라지면 부르던 쪽이 깨진다.
var exportPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?m)^\s*export\s+(?:default\s+)?(?:async\s+)?function\s+([A-Za-z_$][\w$]*)`),
	regexp.MustCompile(`(?m)^\s*export\s+(?:const|let|var|class|interface|type|enum)\s+([A-Za-z_$][\w$]*)`),
	regexp.MustCompile(`(?m)^\s*func\s+([A-Z][\w]*)\s*\(`),
}

// exportedNames 는 파일이 밖으로 내주는 이름을 모은다.
func exportedNames(src string) map[string]bool {
	out := map[string]bool{}
	for _, re := range exportPatterns {
		for _, m := range re.FindAllStringSubmatch(src, -1) {
			out[m[1]] = true
		}
	}
	// export { a, b as c }
	braceRe := regexp.MustCompile(`(?s)export\s*\{([^}]*)\}`)
	for _, m := range braceRe.FindAllStringSubmatch(src, -1) {
		for _, part := range strings.Split(m[1], ",") {
			part = strings.TrimSpace(part)
			if i := strings.LastIndex(strings.ToLower(part), " as "); i >= 0 {
				part = strings.TrimSpace(part[i+4:])
			}
			if part != "" {
				out[part] = true
			}
		}
	}
	return out
}

// lostExports 는 고친 뒤 사라진 이름을 준다.
//
// 통짜로 다시 쓰게 하면 모델이 조용히 함수를 빠뜨린다. 빌드가 깨지고 나서야
// 알게 되는데, 그때는 이미 자가 치유 세 번을 태운 뒤다.
func lostExports(before, after string) []string {
	had := exportedNames(before)
	has := exportedNames(after)
	var lost []string
	for name := range had {
		if !has[name] {
			lost = append(lost, name)
		}
	}
	return lost
}

func clipRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + " …"
}

// replaceLoosely 는 공백 차이를 무시하고 한 군데를 바꾼다.
//
// 줄마다 앞뒤 공백을 턴 것으로 견준다. 딱 한 군데만 맞을 때만 바꾼다 —
// 여러 군데면 어디인지 모르는 것이고, 그때는 바꾸지 않는 편이 낫다.
func replaceLoosely(src, search, replace string) (string, error) {
	srcLines := strings.Split(src, "\n")
	wantLines := trimEach(strings.Split(strings.TrimRight(search, "\n"), "\n"))
	if len(wantLines) == 0 {
		return "", fmt.Errorf("찾을 내용이 비었다")
	}

	var at []int
	for i := 0; i+len(wantLines) <= len(srcLines); i++ {
		ok := true
		for j, w := range wantLines {
			if strings.TrimSpace(srcLines[i+j]) != w {
				ok = false
				break
			}
		}
		if ok {
			at = append(at, i)
		}
	}
	switch len(at) {
	case 1:
		i := at[0]
		out := append([]string{}, srcLines[:i]...)
		out = append(out, strings.Split(strings.TrimRight(replace, "\n"), "\n")...)
		out = append(out, srcLines[i+len(wantLines):]...)
		return strings.Join(out, "\n"), nil
	case 0:
		// 줄바꿈까지 다를 수 있다. 분석이 코드를 한 줄로 펴서 적어 주면
		// 원문의 네 줄과 줄 단위로는 영영 안 맞는다.
		return replaceFlattened(srcLines, search, replace)
	default:
		return "", fmt.Errorf("공백을 무시해도 %d군데가 맞아 어디인지 알 수 없다:\n%s", len(at), clipRunes(search, 200))
	}
}

// replaceFlattened 는 줄바꿈까지 무시하고 한 군데를 바꾼다.
//
// 이어지는 몇 줄을 붙여 공백을 접은 것이 찾는 내용과 같으면 그 줄들을
// 통째로 바꾼다. 여기서도 한 군데일 때만 바꾼다.
func replaceFlattened(srcLines []string, search, replace string) (string, error) {
	want := flattenCode(search)
	if want == "" {
		return "", fmt.Errorf("찾을 내용이 비었다")
	}

	type span struct{ start, end int }
	var found []span
	for i := range srcLines {
		var acc strings.Builder
		for j := i; j < len(srcLines) && j < i+40; j++ {
			acc.WriteString(srcLines[j])
			acc.WriteString(" ")
			got := flattenCode(acc.String())
			if len(got) > len(want) {
				break
			}
			if got == want {
				found = append(found, span{i, j})
				break
			}
		}
	}
	switch len(found) {
	case 1:
		sp := found[0]
		out := append([]string{}, srcLines[:sp.start]...)
		// 원문의 들여쓰기를 그대로 물려준다.
		indent := srcLines[sp.start][:len(srcLines[sp.start])-len(strings.TrimLeft(srcLines[sp.start], " \t"))]
		for _, l := range strings.Split(strings.TrimRight(replace, "\n"), "\n") {
			out = append(out, indent+strings.TrimSpace(l))
		}
		out = append(out, srcLines[sp.end+1:]...)
		return strings.Join(out, "\n"), nil
	case 0:
		return "", fmt.Errorf("원문에 없는 내용을 찾으라고 했다:\n%s", clipRunes(search, 200))
	default:
		return "", fmt.Errorf("줄바꿈을 무시해도 %d군데가 맞아 어디인지 알 수 없다:\n%s", len(found), clipRunes(search, 200))
	}
}

// flattenCode 는 공백과 줄바꿈을 하나로 접는다. 코드의 뜻은 그대로 둔다.
func flattenCode(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func trimEach(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if t := strings.TrimSpace(l); t != "" {
			out = append(out, t)
		}
	}
	return out
}

var lineNumberRe = regexp.MustCompile(`^\s*\d+:\s?`)

// stripLineNumbers 는 줄 앞에 붙은 번호를 뗀다.
//
// 관련 부분을 보여 줄 때 "30: " 처럼 번호를 붙인다. 빼고 적으라고 일러도
// 모델은 그대로 옮겨 적는다 — 실제로 정확한 줄을 짚고도 번호 때문에
// "원문에 없는 내용" 으로 버려졌다. 사람이 시키는 대신 기계가 뗀다.
//
// **모든 줄에 번호가 붙어 있을 때만** 뗀다. 코드 안의 "case 1:" 같은 것을
// 번호로 오인하면 안 된다.
func stripLineNumbers(s string) string {
	lines := strings.Split(s, "\n")
	n := 0
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		if !lineNumberRe.MatchString(l) {
			return s
		}
		n++
	}
	if n == 0 {
		return s
	}
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = lineNumberRe.ReplaceAllString(l, "")
	}
	return strings.Join(out, "\n")
}
