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
		search, replace := m[1], m[2]
		if strings.TrimSpace(search) == "" {
			return "", fmt.Errorf("찾을 내용이 비었다")
		}
		switch strings.Count(out, search) {
		case 1:
			out = strings.Replace(out, search, replace, 1)
		case 0:
			return "", fmt.Errorf("원문에 없는 내용을 찾으라고 했다:\n%s", clipRunes(search, 200))
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
