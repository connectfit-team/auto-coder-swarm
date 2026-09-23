package agent

import (
	"strings"
	"testing"
)

// 모델이 —·「」 를 -·"" 로 받아 적어 자리는 맞췄지만, 손대지 않은 줄까지
// 그 꼴로 덮어써지면 안 된다.
func TestFoldedMatchKeepsOriginalPunctuation(t *testing.T) {
	src := strings.Join([]string{
		"func f() {",
		"\t// 이름이 틀리면 — 에러 없이 「빈 칸」이 렌더된다…",
		"\treturn 1",
		"}",
	}, "\n")
	raw := "<<<<<<< SEARCH\n" +
		"\t// 이름이 틀리면 - 에러 없이 \"빈 칸\"이 렌더된다...\n" +
		"\treturn 1\n" +
		"=======\n" +
		"\t// 이름이 틀리면 - 에러 없이 \"빈 칸\"이 렌더된다...\n" +
		"\treturn 2\n" +
		">>>>>>> REPLACE"

	got, err := applyEditBlocks(src, raw)
	if err != nil {
		t.Fatalf("적용 실패: %v", err)
	}
	if !strings.Contains(got, "return 2") {
		t.Fatal("고치려던 줄이 안 바뀌었다")
	}
	if !strings.Contains(got, "틀리면 — 에러 없이 「빈 칸」이 렌더된다…") {
		t.Fatalf("손대지 않은 줄의 문장부호가 훼손됐다:\n%s", got)
	}
}

// 일부러 바꾼 줄은 모델의 것을 써야 한다 — 되돌리기가 수정을 먹으면 안 된다.
func TestFoldedMatchStillAppliesRealEdits(t *testing.T) {
	src := "\t// 앞머리 — 설명\n\tlimit := 10\n"
	raw := "<<<<<<< SEARCH\n\t// 앞머리 - 설명\n\tlimit := 10\n=======\n" +
		"\t// 앞머리 - 설명\n\tlimit := 20\n>>>>>>> REPLACE"

	got, err := applyEditBlocks(src, raw)
	if err != nil {
		t.Fatalf("적용 실패: %v", err)
	}
	if !strings.Contains(got, "limit := 20") {
		t.Fatalf("수정이 되돌려졌다:\n%s", got)
	}
	if !strings.Contains(got, "앞머리 — 설명") {
		t.Fatalf("주석이 훼손됐다:\n%s", got)
	}
}

// 정확히 맞아 떨어지는 자리는 되돌리기가 끼어들지 않아야 한다.
func TestExactMatchUntouched(t *testing.T) {
	src := "a := 1\nb := 2\n"
	raw := "<<<<<<< SEARCH\nb := 2\n=======\nb := 3\n>>>>>>> REPLACE"
	got, err := applyEditBlocks(src, raw)
	if err != nil {
		t.Fatalf("적용 실패: %v", err)
	}
	if !strings.Contains(got, "b := 3") {
		t.Fatalf("정확히 맞은 자리가 안 바뀌었다:\n%s", got)
	}
}
