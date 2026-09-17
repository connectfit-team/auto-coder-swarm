package agent

import (
	"strings"
	"testing"
)

func TestParseSpot(t *testing.T) {
	ok := map[string]editSpot{
		"104":             {104, "after"},
		"104 뒤":           {104, "after"},
		"104줄 뒤":          {104, "after"},
		"12 앞":            {12, "before"},
		"7 고침":            {7, "replace"},
		"  104 after":     {104, "after"},
		"line 30 replace": {30, "replace"},
	}
	for in, want := range ok {
		got, err := parseSpot(in, 200)
		if err != nil {
			t.Errorf("%q: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("%q → %+v, 바람 %+v", in, got, want)
		}
	}
	// 파일 밖은 거절한다 — 엉뚱한 자리에 넣느니 다시 묻는 것이 낫다.
	for _, bad := range []string{"0", "999", "없다", ""} {
		if _, err := parseSpot(bad, 200); err == nil {
			t.Errorf("%q 를 받아들였다", bad)
		}
	}
}

func TestSpliceSnippet(t *testing.T) {
	src := "a\n\tb\nc\n"
	// 뒤에 넣기 — 들여쓰기를 그 자리에서 물려받는다.
	got := spliceSnippet(src, editSpot{2, "after"}, "x\ny")
	if got != "a\n\tb\n\tx\n\ty\nc\n" {
		t.Errorf("뒤에 넣기: %q", got)
	}
	got = spliceSnippet(src, editSpot{2, "before"}, "x")
	if got != "a\n\tx\n\tb\nc\n" {
		t.Errorf("앞에 넣기: %q", got)
	}
	got = spliceSnippet(src, editSpot{2, "replace"}, "x")
	if got != "a\n\tx\nc\n" {
		t.Errorf("고치기: %q", got)
	}
	// 파일 밖이면 그대로 둔다.
	if spliceSnippet(src, editSpot{99, "after"}, "x") != src {
		t.Error("파일 밖인데 건드렸다")
	}
}

func TestNumberedAll(t *testing.T) {
	got, n := numberedAll("a\nb\nc", 10)
	if n != 3 || !strings.Contains(got, "2: b") {
		t.Errorf("%q n=%d", got, n)
	}
}
