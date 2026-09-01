package agent

import (
	"strings"
	"testing"
)

func TestRelevantRegions(t *testing.T) {
	var b strings.Builder
	for i := 1; i <= 200; i++ {
		if i == 100 {
			b.WriteString("        lt: new Date(new Date().getFullYear(), new Date().getMonth() + 1, 0)\n")
			continue
		}
		b.WriteString("const filler = 1;\n")
	}
	got := relevantRegions(b.String(), "getMonth 를 쓰는 곳에서 말일이 빠진다. new Date 경계를 고쳐라")
	if got == "" {
		t.Fatal("관련 부분을 못 찾았다")
	}
	if !strings.Contains(got, "getMonth") {
		t.Errorf("고칠 줄이 빠졌다:\n%s", got)
	}
	if !strings.Contains(got, "100: ") {
		t.Errorf("줄 번호가 없다:\n%s", got)
	}
	// 200줄 중 일부만 나와야 한다.
	if n := strings.Count(got, "\n"); n > regionMaxLines {
		t.Errorf("너무 많이 준다: %d줄", n)
	}
}

func TestRelevantRegionsGivesUpWhenEverythingMatches(t *testing.T) {
	src := strings.Repeat("const value = 1;\n", 300)
	// 모든 줄에 걸리면 잘라 주는 뜻이 없다 — 빈 문자열을 주고 부르는 쪽이 판단한다.
	if got := relevantRegions(src, "value 를 고쳐라"); got != "" {
		t.Errorf("전부 걸렸는데 잘라 줬다: %d바이트", len(got))
	}
}

func TestRelevantRegionsNoIdentifiers(t *testing.T) {
	if got := relevantRegions("a\nb\n", "고쳐라"); got != "" {
		t.Errorf("이름이 없는데 뭔가 줬다: %q", got)
	}
}
