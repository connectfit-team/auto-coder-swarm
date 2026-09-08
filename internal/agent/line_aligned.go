package agent

import "strings"

// 글자 단위로 바꾸면 토큰이 잘린다.
//
// 블록 적용은 `strings.Replace(out, search, replace, 1)` 였다. 찾는 내용이
// 줄 경계에 맞지 않으면 낱말 가운데를 자른다 — 실측으로 `buffers` 가
// `ffers` 로, `new` 가 `ew` 로 남은 파일이 나왔고 오류가 234개였다
// (W-43067 · W-91980).
//
// 모델에게는 "줄을 그대로 옮겨 적어라" 라고 이른다. 그러니 **줄 경계에
// 맞는 것만** 바꾼다. 안 맞으면 줄 단위로 견주는 뒤의 길로 넘긴다 —
// 그 길들은 줄을 통째로 갈아 끼우므로 낱말을 자르지 않는다.

// lineAlignedCount 는 줄 경계에 맞는 자리가 몇 곳인지 센다.
func lineAlignedCount(hay, needle string) int {
	if needle == "" {
		return 0
	}
	n := 0
	for i := 0; ; {
		j := strings.Index(hay[i:], needle)
		if j < 0 {
			return n
		}
		at := i + j
		if lineAligned(hay, at, len(needle)) {
			n++
		}
		i = at + 1
	}
}

// replaceLineAligned 는 줄 경계에 맞는 자리만 바꾼다.
// all 이 false 면 첫 자리만 바꾼다.
func replaceLineAligned(hay, needle, repl string, all bool) string {
	if needle == "" {
		return hay
	}
	var b strings.Builder
	i := 0
	for {
		j := strings.Index(hay[i:], needle)
		if j < 0 {
			b.WriteString(hay[i:])
			return b.String()
		}
		at := i + j
		if !lineAligned(hay, at, len(needle)) {
			b.WriteString(hay[i : at+1])
			i = at + 1
			continue
		}
		b.WriteString(hay[i:at])
		b.WriteString(repl)
		i = at + len(needle)
		if !all {
			b.WriteString(hay[i:])
			return b.String()
		}
	}
}

// lineAligned 는 그 자리가 줄의 시작에서 시작해 줄의 끝에서 끝나는지 본다.
//
// 앞에는 줄머리이거나 줄바꿈만 있어야 하고(들여쓰기는 찾는 내용에 포함돼
// 있을 수 있으므로 빈칸도 허용한다), 뒤에는 줄바꿈이거나 파일 끝이어야 한다.
func lineAligned(hay string, at, n int) bool {
	// 앞: 줄머리까지 빈칸만 있어야 한다.
	for i := at - 1; i >= 0; i-- {
		switch hay[i] {
		case '\n':
			goto tail
		case ' ', '\t':
			continue
		default:
			return false
		}
	}
tail:
	end := at + n
	if end >= len(hay) {
		return true
	}
	// 뒤: 줄끝까지 빈칸만 있어야 한다.
	for i := end; i < len(hay); i++ {
		switch hay[i] {
		case '\n':
			return true
		case ' ', '\t', '\r':
			continue
		default:
			return false
		}
	}
	return true
}
