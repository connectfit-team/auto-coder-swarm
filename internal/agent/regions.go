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
	// 예산을 못 재는 자리(시험 따위)에서 쓰는 줄 수 상한.
	// 실제 호출은 창 예산에서 역산한 값을 넘긴다.
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
	return relevantRegionsWithin(original, instructions, regionMaxLines)
}

// relevantRegionsWithin 은 보여 줄 줄 수의 상한을 받는다.
func relevantRegionsWithin(original, instructions string, maxLines int) string {
	if maxLines < 40 {
		maxLines = 40
	}
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
	if len(keep) == 0 {
		return ""
	}
	// 너무 많이 걸리면 창을 좁혀 다시 고른다. 포기하고 통째로 넘기면
	// 프롬프트가 창을 넘어 400 이 난다.
	if len(keep) > maxLines {
		keep = narrowTo(lines, wanted, maxLines)
		if len(keep) == 0 {
			return ""
		}
	}

	idx := make([]int, 0, len(keep))
	for i := range keep {
		idx = append(idx, i)
	}
	sort.Ints(idx)

	var b strings.Builder
	prev := -2
	for _, i := range idx {
		// **생략을 분명히 적는다.**
		//
		// `...` 한 줄로는 모델이 무시한다. 실제로 5줄과 그 아래 어딘가를
		// 이어 붙여 SEARCH 를 만들었고, 그런 줄은 원문에 없으니 고치기가
		// 통째로 실패했다(W-31838).
		//
		//	import { t } from '$lib/i18n/context';   ← 5줄
		//	                                          ← 원문에는 다른 줄들이 있다
		//	const { t } = useI18n();                 ← 한참 아래
		//
		// 몇 줄이 빠졌는지, 이어진 줄이 아니라는 것을 적으면 붙일 수 없다.
		if i != prev+1 && prev >= 0 {
			fmt.Fprintf(&b, "…… %d~%d줄은 보여 주지 않았다. **이 위와 아래는 이어진 줄이 아니다** ……\n",
				prev+2, i)
		}
		// 줄 번호를 붙여 어디인지 알게 한다. SEARCH 에는 번호를 빼고 적으라고 이른다.
		fmt.Fprintf(&b, "%d: %s\n", i+1, lines[i])
		prev = i
	}
	return b.String()
}

// narrowTo 는 앞뒤 여유를 줄여 가며 보여 줄 줄 수를 맞춘다.
func narrowTo(lines []string, wanted map[string]bool, max int) map[int]bool {
	for ctx := regionContext - 3; ctx >= 1; ctx -= 3 {
		keep := map[int]bool{}
		for i, ln := range lines {
			low := strings.ToLower(ln)
			for w := range wanted {
				if strings.Contains(low, w) {
					for j := i - ctx; j <= i+ctx; j++ {
						if j >= 0 && j < len(lines) {
							keep[j] = true
						}
					}
					break
				}
			}
		}
		if len(keep) > 0 && len(keep) <= max {
			return keep
		}
	}
	return nil
}
