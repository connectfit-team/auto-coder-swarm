package agent

import (
	"strings"
	"testing"
)

// 겹치는 자리를 알려 주지 않으면 모델은 같은 블록을 또 낸다.
func TestAmbiguousTellsWhere(t *testing.T) {
	src := strings.Join([]string{
		"function a() {",
		"  const v = 1",
		"}",
		"function b() {",
		"  const v = 1",
		"}",
	}, "\n")
	raw := "<<<<<<< SEARCH\n  const v = 1\n=======\n  const v = 2\n>>>>>>> REPLACE"

	_, err := applyEditBlocks(src, raw)
	if err == nil {
		t.Fatal("두 군데가 맞는데 그냥 고쳤다")
	}
	msg := err.Error()
	for _, want := range []string{"2군데", "L2:", "L5:", "SEARCH 를 길게"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("%q 가 없다:\n%s", want, msg)
		}
	}
}

func TestLocationsCapAtFive(t *testing.T) {
	lines := make([]string, 0, 9)
	at := make([]int, 0, 9)
	for i := 0; i < 9; i++ {
		lines = append(lines, "  x = do()")
		at = append(at, i)
	}
	got := matchLocations(lines, at)
	if n := strings.Count(got, "  L"); n != 5 {
		t.Fatalf("다섯 줄만 보여야 하는데 %d줄이다:\n%s", n, got)
	}
	if !strings.Contains(got, "4군데 더") {
		t.Fatalf("넘친 수를 안 알렸다:\n%s", got)
	}
}

func TestLongLinesClipped(t *testing.T) {
	got := matchLocations([]string{"y = " + strings.Repeat("z", 300)}, []int{0})
	for _, ln := range strings.Split(got, "\n") {
		if len([]rune(ln)) > 100 {
			t.Fatalf("줄이 너무 길다(%d자): %s", len([]rune(ln)), ln)
		}
	}
}

// 자리 문제와 「그 파일은 못 고친다」 를 갈라야 한다.
func TestEditBlockProblemIsMarked(t *testing.T) {
	src := "a := 1\nb := 1\n"
	raw := "<<<<<<< SEARCH\n없는 줄이다\n=======\nx\n>>>>>>> REPLACE"
	if _, err := applyEditBlocks(src, raw); err == nil {
		t.Fatal("없는 줄을 찾았다고 한다")
	}
	if IsEditBlockProblem(nil) {
		t.Fatal("nil 을 자리 문제라고 한다")
	}
	wrapped := &editBlockProblem{err: errFake{}}
	if !IsEditBlockProblem(wrapped) {
		t.Fatal("표지를 못 알아본다")
	}
	if IsEditBlockProblem(errFake{}) {
		t.Fatal("아무 오류나 자리 문제라고 한다")
	}
}

type errFake struct{}

func (errFake) Error() string { return "그냥 오류" }
