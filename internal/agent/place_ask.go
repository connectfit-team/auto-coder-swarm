package agent

import (
	"context"
	"fmt"
	"strings"
)

// 한 번에 하나만 묻는다.
//
// 자리를 고르는 물음에는 **번호 하나**만 답하면 된다. 코드를 쓰는 물음에는
// **코드만** 쓰면 된다. 형식을 맞추는 일은 없다 — 기계가 붙인다.

const maxPlacementLines = 400

// editByPlacement 는 찾아바꾸기가 실패했을 때 쓰는 다른 길이다.
func (a *CoderAgent) editByPlacement(ctx context.Context, filePath, original, instructions string) (string, error) {
	numbered, total := numberedAll(original, maxPlacementLines)
	if total == 0 {
		return "", fmt.Errorf("빈 파일이다")
	}
	shown := total
	if shown > maxPlacementLines {
		shown = maxPlacementLines
	}

	spot, err := a.askSpot(ctx, filePath, numbered, instructions, shown)
	if err != nil {
		return "", err
	}

	snippet, err := a.askSnippet(ctx, filePath, original, spot, instructions)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(snippet) == "" {
		return "", fmt.Errorf("넣을 코드를 내지 않았다")
	}
	return spliceSnippet(original, spot, snippet), nil
}

// askSpot 은 **자리 하나**만 묻는다.
func (a *CoderAgent) askSpot(ctx context.Context, filePath, numbered, instructions string, maxLine int) (editSpot, error) {
	prompt := fmt.Sprintf(`아래는 %s 의 내용이다. 앞의 숫자는 줄 번호다.

%s
[할 일]
%s

이 일을 하려면 **어디를 건드려야 하나?** 줄 번호 하나만 답해라.

  새 코드를 넣어야 하면   "<번호> 뒤"  또는 "<번호> 앞"
  있는 줄을 고쳐야 하면   "<번호> 고침"

다른 말은 쓰지 마라. 보기: 104 뒤`, filePath, numbered, instructions)

	raw, err := CallLLM(ctx, a.llm, a.Name(), prompt)
	if err != nil {
		return editSpot{}, err
	}
	spot, perr := parseSpot(raw, maxLine)
	if perr == nil {
		return spot, nil
	}
	// 한 번 더 짧게 묻는다. 긴 답을 낸 것뿐인 경우가 많다.
	raw2, err2 := CallLLM(ctx, a.llm, a.Name(),
		"줄 번호 하나만 답해라. 보기: 104 뒤\n\n[할 일]\n"+instructions+"\n\n"+numbered)
	if err2 != nil {
		return editSpot{}, perr
	}
	return parseSpot(raw2, maxLine)
}

// askSnippet 은 **그 자리에 들어갈 코드**만 묻는다.
func (a *CoderAgent) askSnippet(ctx context.Context, filePath, original string, spot editSpot, instructions string) (string, error) {
	around := aroundLines(original, spot.line, 12)
	what := "그 줄 뒤에 이어질"
	switch spot.mode {
	case "before":
		what = "그 줄 앞에 올"
	case "replace":
		what = "그 줄을 대신할"
	}

	prompt := fmt.Sprintf(`%s 의 %d번째 줄 둘레다.

%s
[할 일]
%s

%s 코드만 써라.

  · 설명하지 마라. 코드만 내라.
  · 들여쓰기는 신경 쓰지 마라 — 기계가 맞춘다.
  · 둘레에 이미 있는 이름만 써라. 없는 이름을 지어내지 마라.`,
		filePath, spot.line, around, instructions, what)

	raw, err := CallLLM(ctx, a.llm, a.Name(), prompt)
	if err != nil {
		return "", err
	}
	return CleanCodeOutput(raw), nil
}

// aroundLines 는 그 줄 둘레를 번호와 함께 준다.
func aroundLines(src string, line, span int) string {
	lines := strings.Split(src, "\n")
	lo, hi := line-span, line+span
	if lo < 1 {
		lo = 1
	}
	if hi > len(lines) {
		hi = len(lines)
	}
	var b strings.Builder
	for i := lo; i <= hi; i++ {
		mark := "   "
		if i == line {
			mark = "→  "
		}
		fmt.Fprintf(&b, "%s%d: %s\n", mark, i, lines[i-1])
	}
	return b.String()
}
