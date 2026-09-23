package agent

import (
	"fmt"
	"strings"
)

// 겹치는 자리를 몇 군데까지 보여 줄지. 다 보여 주면 프롬프트만 길어진다.
const maxShownLocations = 5

// matchLocations 는 찾는 내용이 걸린 자리를 줄 번호와 함께 적는다.
//
// "여러 번 나오지만 너무 짧아 어디인지 알 수 없다" 고만 하면 모델은 같은
// 블록을 또 낸다 — 무엇을 고쳐야 할지 모르기 때문이다. 어디에서 겹치는지
// 보여 주면 구분할 줄을 앞뒤로 더 넣을 수 있다.
func matchLocations(lines []string, at []int) string {
	if len(at) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%d군데에서 걸린다:\n", len(at)))
	for i, idx := range at {
		if i >= maxShownLocations {
			fmt.Fprintf(&b, "  ... 그리고 %d군데 더\n", len(at)-maxShownLocations)
			break
		}
		text := ""
		if idx >= 0 && idx < len(lines) {
			text = strings.TrimSpace(lines[idx])
		}
		fmt.Fprintf(&b, "  L%d: %s\n", idx+1, clipRunes(text, 80))
	}
	b.WriteString("앞뒤로 이 자리들을 가르는 줄을 더 넣어 SEARCH 를 길게 적어라.")
	return b.String()
}

// alignedMatchLines 는 줄 경계에 맞는 자리들의 줄 번호(0부터)를 준다.
func alignedMatchLines(hay, needle string) []int {
	if needle == "" {
		return nil
	}
	var at []int
	for i := 0; ; {
		j := strings.Index(hay[i:], needle)
		if j < 0 {
			return at
		}
		pos := i + j
		if lineAligned(hay, pos, len(needle)) {
			at = append(at, strings.Count(hay[:pos], "\n"))
		}
		i = pos + 1
	}
}
