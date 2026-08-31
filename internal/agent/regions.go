package agent

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const (
	// 고칠 자리 앞뒤로 함께 보여 줄 줄 수.
	regionContext = 12
	// 보여 줄 줄 수의 상한. 이보다 많으면 통째로 보는 것과 다를 바 없다.
	regionMaxLines = 140
)

var identifierRe = regexp.MustCompile(`[A-Za-z_$][A-Za-z0-9_$]{2,}`)

// relevantRegions 는 지시문과 관련된 부분만 잘라 준다.
//
// 426줄짜리 파일을 통째로 읽히고 "고칠 자리만 내라" 고 하면 작은 모델은 길을
// 잃는다 — 실측으로 형식을 무시하고 잘린 파일을 냈다. 지시문에 나온 이름이
// 있는 줄 둘레만 보여 주면 볼 것이 줄고 어디를 고칠지가 분명해진다.
//
// 관련 줄을 못 찾으면 빈 문자열을 준다. 부르는 쪽이 통째로 보여 주면 된다.
func relevantRegions(original, instructions string) string {
	lines := strings.Split(original, "\n")

	wanted := map[string]bool{}
	for _, m := range identifierRe.FindAllString(instructions, -1) {
		wanted[strings.ToLower(m)] = true
	}
	if len(wanted) == 0 {
		return ""
	}

	keep := map[int]bool{}
	for i, ln := range lines {
		low := strings.ToLower(ln)
		for w := range wanted {
			if strings.Contains(low, w) {
				for j := i - regionContext; j <= i+regionContext; j++ {
					if j >= 0 && j < len(lines) {
						keep[j] = true
					}
				}
				break
			}
		}
	}
	if len(keep) == 0 || len(keep) > regionMaxLines {
		return ""
	}

	idx := make([]int, 0, len(keep))
	for i := range keep {
		idx = append(idx, i)
	}
	sort.Ints(idx)

	var b strings.Builder
	prev := -2
	for _, i := range idx {
		if i != prev+1 && prev >= 0 {
			b.WriteString("...\n")
		}
		// 줄 번호를 붙여 어디인지 알게 한다. SEARCH 에는 번호를 빼고 적으라고 이른다.
		fmt.Fprintf(&b, "%d: %s\n", i+1, lines[i])
		prev = i
	}
	return b.String()
}
