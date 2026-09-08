package agent

import (
	"strings"
	"testing"
)

// 낱말 가운데를 자르면 안 된다. W-43067 이 이것으로 파일 넷을 뭉갰다.
func TestReplaceDoesNotCutTokens(t *testing.T) {
	src := "package p\n\nfunc f() {\n\tbuffers := newBuffer()\n\treturn buffers\n}\n"
	// 모델이 낱말 조각을 찾으라고 낸 경우.
	if n := lineAlignedCount(src, "bu"); n != 0 {
		t.Errorf("낱말 조각을 %d곳 세었다 — 자르게 된다", n)
	}
	got := replaceLineAligned(src, "bu", "X", true)
	if got != src {
		t.Errorf("낱말 가운데를 잘랐다:\n%s", got)
	}

	// 줄 하나를 온전히 주면 바꾼다.
	line := "\tbuffers := newBuffer()"
	if n := lineAlignedCount(src, line); n != 1 {
		t.Fatalf("온전한 줄을 %d곳 세었다", n)
	}
	out := replaceLineAligned(src, line, "\tbuffers := newPool()", false)
	if !strings.Contains(out, "newPool()") || strings.Contains(out, "newBuffer()") {
		t.Errorf("줄을 못 바꿨다:\n%s", out)
	}
	// 다른 줄은 그대로여야 한다.
	if !strings.Contains(out, "return buffers") {
		t.Errorf("다른 줄이 망가졌다:\n%s", out)
	}
}

// 여러 줄도 줄 경계에 맞으면 바꾼다.
func TestReplaceMultiLineAligned(t *testing.T) {
	src := "package p\n\nif a {\n\tb()\n}\n\nif a {\n\tb()\n}\n"
	search := "if a {\n\tb()\n}"
	if n := lineAlignedCount(src, search); n != 2 {
		t.Fatalf("두 곳인데 %d곳 세었다", n)
	}
	out := replaceLineAligned(src, search, "if c {\n\td()\n}", true)
	if strings.Contains(out, "b()") {
		t.Errorf("둘 다 안 바꿨다:\n%s", out)
	}
}
