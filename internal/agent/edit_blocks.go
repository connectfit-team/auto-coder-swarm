package agent

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// 원본을 통째로 다시 쓰게 해도 되는 크기. 글자 수 기준이다.
//
// 창이 16,384 토큰이고 출력 몫이 4,096 이다. 원본을 넣고 **다시 전부 출력**
// 하게 하면 같은 내용이
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
4. 고치라고 한 것만 고쳐라. 블록은 여러 개여도 된다.
5. SEARCH 는 **원문에서 붙어 있는 줄**이어야 한다. 「이어진 줄이 아니다」 라고
   적힌 자리를 건너뛰어 위아래를 이어 붙이지 마라 — 그런 줄은 원문에 없다.
   보이지 않는 대목을 고쳐야 하면, 지어내지 말고 무엇이 안 보이는지 적어라.`

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
	applied := 0
	for _, m := range ms {
		search := stripLineNumbers(stripCodeFence(m[1]))
		replace := stripLineNumbers(stripCodeFence(m[2]))
		if strings.TrimSpace(search) == "" {
			return "", fmt.Errorf("찾을 내용이 비었다")
		}
		// **줄 경계에 맞는 자리만 센다.**
		//
		// 글자 단위로 바꾸면 낱말 가운데가 잘린다 — 실측으로 buffers 가
		// ffers 로, new 가 ew 로 남은 파일이 나왔고 오류가 234개였다
		// (W-43067 · W-91980). 안 맞으면 아래의 줄 단위 길로 넘긴다.
		switch n := lineAlignedCount(out, search); {
		case n == 1:
			out = replaceLineAligned(out, search, replace, false)
			applied++
			continue
		case n > 1 && isSubstantial(search):
			// **같은 코드가 여러 곳에 복사돼 있으면 전부 고친다.**
			//
			// 찾는 내용이 글자 그대로 같은 자리들이라 한 곳만 고치면 나머지는
			// 그대로 남는다. 실제로 workplace.ts 의 말일 경계 버그가 29행과
			// 133행 두 곳에 복사돼 있었고, 한 곳만 고치면 반만 고친 것이다.
			out = replaceLineAligned(out, search, replace, true)
			applied++
			continue
		case n > 1:
			return "", fmt.Errorf("원문에 여러 번 나오지만 너무 짧아 어디인지 알 수 없다:\n%s\n\n%s",
				clipRunes(search, 200),
				matchLocations(strings.Split(out, "\n"), alignedMatchLines(out, search)))
		default:
			// **들여쓰기까지 똑같이 옮겨 적기를 바랄 수는 없다.**
			//
			// 모델이 자리는 정확히 짚고도 앞 공백이 달라 못 찾는 일이 잦다.
			// 줄 끝 공백을 털고 줄 단위로 견줘 한 군데만 맞으면 그 자리를 쓴다.
			replaced, err := replaceLoosely(out, search, replace)
			if err != nil {
				// **이미 고친 자리를 또 고치라는 블록은 그냥 넘긴다.**
				//
				// 같은 코드가 두 곳에 복사돼 있으면 모델이 블록을 두 번 낸다.
				// 첫 블록이 두 곳을 다 고치고 나면 두 번째 블록은 찾을 것이
				// 없다. 그걸 오류로 보면 **맞게 고친 것까지 통째로 버려진다** —
				// 실제로 workplace.ts 의 말일 경계 수정이 그렇게 날아갔다.
				if applied > 0 && isNotFound(err) {
					continue
				}
				return "", err
			}
			out = replaced
			applied++
		}
	}
	if out == original {
		return "", fmt.Errorf("블록을 적용했지만 바뀐 것이 없다")
	}
	return out, nil
}

func clipRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + " …"
}

// replaceLoosely 는 공백 차이를 무시하고 바꾼다.
//
// 줄마다 앞뒤 공백을 턴 것으로 견준다. 여러 군데가 맞으면 — 같은 코드가
// 복사돼 있다는 뜻이므로 — 전부 바꾼다. 다만 찾는 내용이 너무 짧으면
// 어디인지 모르는 것이라 바꾸지 않는다.
func replaceLoosely(src, search, replace string) (string, error) {
	srcLines := strings.Split(src, "\n")
	wantLines := trimEach(strings.Split(strings.TrimRight(search, "\n"), "\n"))
	if len(wantLines) == 0 {
		return "", fmt.Errorf("찾을 내용이 비었다")
	}

	folded := false
	at := findTrimmed(srcLines, wantLines, func(s string) string { return s })
	if len(at) == 0 {
		// **문장부호를 고른 꼴로 맞춰 한 번 더 본다.**
		//
		// 이 저장소의 주석에는 —·「」·… 가 흔한데, 모델은 그것을 -·"" 로
		// 받아 적는다. 글자 하나 때문에 「원문에 없는 줄」 이 된다.
		// codex 도 마지막 단에서 같은 것을 한다(seek_sequence.rs).
		at = findTrimmed(srcLines, wantLines, normalizePunct)
		folded = len(at) > 0
	}

	switch {
	case len(at) == 0:
		// 줄바꿈까지 다를 수 있다. 분석이 코드를 한 줄로 펴서 적어 주면
		// 원문의 네 줄과 줄 단위로는 영영 안 맞는다.
		return replaceFlattened(srcLines, search, replace)
	case len(at) > 1 && !isSubstantial(search):
		return "", fmt.Errorf("공백을 무시하면 여러 군데가 맞는데 너무 짧아 어디인지 알 수 없다:\n%s\n\n%s",
			clipRunes(search, 200), matchLocations(srcLines, at))
	}

	spans := make([]int, len(at))
	for i := range spans {
		spans[i] = len(wantLines)
	}
	if folded {
		replace = restorePunct(srcLines[at[0]:at[0]+len(wantLines)], replace)
	}
	return spliceAll(srcLines, at, spans, replace), nil
}

// restorePunct 는 문장부호만 다른 줄을 원문 글자로 되돌린다.
//
// 문장부호를 고른 꼴로 맞춰 **찾은** 자리에는 모델이 낸 줄이 그대로 들어간다.
// 그런데 모델이 —·「」·… 를 -·""·... 로 받아 적었기 때문에 그 자리가 맞은
// 것이므로, 손대려 하지 않은 줄까지 그 꼴로 덮어써진다. 빌드도 lostExports 도
// 이것은 못 잡는다 — 한글 문구라면 사용자에게 보이는 글자가 조용히 바뀐다.
//
// 그래서 바꿀 줄이 원문의 어느 줄과 **문장부호만 빼고 같으면** 원문 글자를
// 쓴다. 들여쓰기는 모델의 것을 둔다 — 그건 일부러 바꿨을 수 있다.
func restorePunct(origSpan []string, replace string) string {
	byFold := make(map[string]string, len(origSpan))
	for _, ln := range origSpan {
		t := strings.TrimSpace(ln)
		if t != "" {
			byFold[normalizePunct(t)] = t
		}
	}
	lines := strings.Split(replace, "\n")
	for i, ln := range lines {
		t := strings.TrimSpace(ln)
		if t == "" {
			continue
		}
		orig, ok := byFold[normalizePunct(t)]
		if !ok || orig == t {
			continue
		}
		lines[i] = ln[:len(ln)-len(strings.TrimLeft(ln, " \t"))] + orig
	}
	return strings.Join(lines, "\n")
}

// replaceFlattened 는 줄바꿈까지 무시하고 바꾼다.
//
// 이어지는 몇 줄을 붙여 공백을 접은 것이 찾는 내용과 같으면 그 줄들을
// 통째로 바꾼다. 여러 군데면 — 복사된 코드라는 뜻이므로 — 전부 바꾼다.
// appendedNote 는 마지막으로 붙인 선언에 대한 말이다. 로그에 남기려고
// 둔다 — 붙인 것을 바꾼 것처럼 보고하면 사람이 diff 를 잘못 읽는다.
var appendedNote string

// AppendedNote 는 방금 붙인 선언에 대한 말을 준다.
func AppendedNote() string { return appendedNote }

func replaceFlattened(srcLines []string, search, replace string) (string, error) {
	want := flattenCode(search)
	if want == "" {
		return "", fmt.Errorf("찾을 내용이 비었다")
	}

	var at, spans []int
	for i := 0; i < len(srcLines); i++ {
		var acc strings.Builder
		for j := i; j < len(srcLines) && j < i+40; j++ {
			acc.WriteString(srcLines[j])
			acc.WriteString(" ")
			got := flattenCode(acc.String())
			if len(got) > len(want) {
				break
			}
			if got == want {
				at = append(at, i)
				spans = append(spans, j-i+1)
				i = j // 겹쳐 세지 않는다
				break
			}
		}
	}

	switch {
	case len(at) == 0:
		// **새 선언은 찾을 것이 아니라 붙일 것이다.**
		//
		// 없는 기능을 만들 때 코더는 "// X 메서드를 구현합니다" 를 찾으라고
		// 한다. 원문에 있을 수가 없다. 형제 옆에 붙이면 되는 일이다
		// (실측 W-57730 이 이것으로 세 시도를 태웠다).
		if out, why, ok := appendBesideSibling(srcLines, search, replace); ok {
			appendedNote = why
			return out, nil
		}
		// 원문에 무엇이 있는지 함께 준다. 없다고만 말하면 다시 계획해도
		// 같은 것을 지어낸다(실측 W-70980: 세 시도가 같은 자리에서 죽었다).
		msg := fmt.Sprintf("원문에 없는 내용을 찾으라고 했다:\n%s", clipRunes(search, 200))
		if near := nearestAnchor(srcLines, search); near != "" {
			msg += "\n" + near
			// **어떻게 하라는지까지 적는다.**
			//
			// "없다" 와 "가장 비슷한 곳은 104줄" 만으로는 같은 실수를 되풀이한다.
			// 실측으로 한 파일이 두 번 다 **새로 넣을 줄을 SEARCH 에 적어**
			// 실패했다(W-63343). 넣으려는 것과 찾으려는 것을 헷갈린 것이다.
			//
			// 그 파일의 실제 줄로 본보기를 만들어 보여 준다.
			if anchor := firstAnchorLine(near); anchor != "" {
				msg += "\n\n새 줄을 **넣으려는** 것이면 SEARCH 에 적을 것은 이미 있는 줄이다." +
					"\n그 줄을 SEARCH 에 두고, REPLACE 에 그 줄과 새 줄을 함께 적어라:" +
					"\n<<<<<<< SEARCH\n" + anchor +
					"\n=======\n" + anchor + "\n<여기에 새 줄>\n>>>>>>> REPLACE"
			}
		} else {
			msg += "\n겹치는 이름이 거의 없다 — 이 파일이 아닐 수 있다."
		}
		return "", fmt.Errorf("%s", msg)
	case len(at) > 1 && !isSubstantial(search):
		return "", fmt.Errorf("줄바꿈을 무시하면 %d군데가 맞는데 너무 짧아 어디인지 알 수 없다:\n%s",
			len(at), clipRunes(search, 200))
	}
	return spliceAll(srcLines, at, spans, replace), nil
}

// spliceAll 은 찾은 자리들을 **뒤에서부터** 바꾼다.
//
// 앞에서부터 바꾸면 줄 번호가 밀려 다음 자리를 잘못 짚는다. 바꿔 넣는 줄에는
// 원문의 들여쓰기를 물려준다.
func spliceAll(srcLines []string, at, spans []int, replace string) string {
	repl := strings.Split(strings.TrimRight(replace, "\n"), "\n")
	out := append([]string{}, srcLines...)
	for k := len(at) - 1; k >= 0; k-- {
		i, span := at[k], spans[k]
		block := reindent(repl, leadingSpace(out[i]))
		next := append([]string{}, out[:i]...)
		next = append(next, block...)
		next = append(next, out[i+span:]...)
		out = next
	}
	return strings.Join(out, "\n")
}

func leadingSpace(s string) string {
	return s[:len(s)-len(strings.TrimLeft(s, " \t"))]
}

// isSubstantial 은 찾는 내용이 자리를 특정할 만큼 긴지 본다.
//
// "}" 한 글자가 여러 군데 맞는다고 전부 바꾸면 파일이 망가진다.
func isSubstantial(search string) bool {
	return len(flattenCode(search)) >= 20
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

var codeFenceRe = regexp.MustCompile("(?m)^\\s*```[a-zA-Z0-9_+-]*\\s*$")

// stripCodeFence 는 블록 안에 딸려 온 마크다운 울타리를 뗀다.
//
// 모델이 코드를 적을 때 습관처럼 ```typescript 을 붙인다. 그 줄은 원문에
// 없으므로 그대로 두면 영영 못 찾는다 — 실측으로 정확한 코드를 짚고도
// 울타리 때문에 버려졌다.
func stripCodeFence(s string) string {
	if !strings.Contains(s, "```") {
		return s
	}
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if codeFenceRe.MatchString(l) {
			continue
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

// isNotFound 는 "찾을 것이 없다" 류의 실패인지 본다.
//
// 여러 군데라 못 정하겠다는 것과는 다르다. 그건 넘기면 안 된다.
func isNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "원문에 없는 내용")
}

// reindent 는 바꿔 넣을 블록을 원문 자리의 들여쓰기에 맞춘다.
//
// **블록 안의 상대 들여쓰기는 지킨다.** 예전에는 모든 줄에 첫 줄 들여쓰기를
// 그대로 붙였다. 그래서 workAt: { 안의 gte·lt 가 바깥과 같은 깊이로 나와
// 들여쓰기가 무너졌다(16칸 → 12칸). 동작에는 지장이 없지만 리뷰에서 지적당한다.
func reindent(lines []string, indent string) []string {
	base := -1
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		if w := len(leadingSpace(l)); base < 0 || w < base {
			base = w
		}
	}
	if base < 0 {
		base = 0
	}
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			out = append(out, "")
			continue
		}
		rel := ""
		if w := len(leadingSpace(l)); w > base {
			rel = strings.Repeat(" ", w-base)
		}
		out = append(out, indent+rel+strings.TrimSpace(l))
	}
	return out
}

// firstAnchorLine 은 nearestAnchor 가 보여 준 줄에서 **원문 그대로의 한 줄**을
// 뽑는다. 앞에 붙은 줄 번호는 뗀다 — SEARCH 에는 번호가 들어가면 안 된다.
func firstAnchorLine(near string) string {
	for _, ln := range strings.Split(near, "\n") {
		i := strings.Index(ln, ":")
		if i <= 0 {
			continue
		}
		if _, err := strconv.Atoi(strings.TrimSpace(ln[:i])); err != nil {
			continue
		}
		if body := strings.TrimRight(ln[i+1:], " \t\r"); strings.TrimSpace(body) != "" {
			return strings.TrimPrefix(body, " ")
		}
	}
	return ""
}

// findTrimmed 는 줄마다 앞뒤 공백을 턴 것으로 견줘 맞는 자리를 준다.
// norm 으로 한 번 더 고른 꼴로 만들 수 있다.
func findTrimmed(srcLines, wantLines []string, norm func(string) string) []int {
	var at []int
	for i := 0; i+len(wantLines) <= len(srcLines); i++ {
		ok := true
		for j, w := range wantLines {
			if norm(strings.TrimSpace(srcLines[i+j])) != norm(w) {
				ok = false
				break
			}
		}
		if ok {
			at = append(at, i)
		}
	}
	return at
}

// 모델이 자주 바꿔 적는 문장부호. 뜻은 같고 글자만 다르다.
var punctFolds = strings.NewReplacer(
	"\u2010", "-", "\u2011", "-", "\u2012", "-", "\u2013", "-", "\u2014", "-", "\u2015", "-", "\u2212", "-",
	"\u2018", "'", "\u2019", "'", "\u201a", "'", "\u201b", "'",
	"\u201c", `"`, "\u201d", `"`, "\u201e", `"`, "\u201f", `"`,
	"\u300c", `"`, "\u300d", `"`, "\u300e", `"`, "\u300f", `"`,
	"\u00b7", "·", "\u2027", "·", "\u30fb", "·",
	"\u2026", "...", "\u00a0", " ",
)

// normalizePunct 는 뜻이 같은 문장부호를 한 꼴로 맞춘다.
func normalizePunct(s string) string { return punctFolds.Replace(s) }
