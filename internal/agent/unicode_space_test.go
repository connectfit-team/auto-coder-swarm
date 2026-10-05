package agent

import (
	"strings"
	"testing"
)

// 공백 갈래(Zs)가 빠지면 그런 글자가 든 파일은 정확한 단에서 다 지고
// 닮은 정도로 찾는 아랫단까지 떨어진다. 　 은 한글 글에 흔하다.
func TestUnicodeSpacesFold(t *testing.T) {
	for _, sp := range []string{" ", " ", " ", " ", " ", " ", "　"} {
		if got := normalizePunct("연결" + sp + "보류"); got != "연결 보류" {
			t.Fatalf("%q 를 못 폈다: %q", sp, got)
		}
	}
}

// 전각 공백이 든 줄을 보통 공백으로 적어 와도 그 자리를 찾아야 한다.
func TestIdeographicSpaceInSource(t *testing.T) {
	src := "// 연결　보류 상태다\nvalue := 1\n"
	raw := "<<<<<<< SEARCH\n// 연결 보류 상태다\nvalue := 1\n=======\n// 연결 보류 상태다\nvalue := 2\n>>>>>>> REPLACE"
	got, err := applyEditBlocks(src, raw)
	if err != nil {
		t.Fatalf("전각 공백 때문에 못 찾았다: %v", err)
	}
	if !strings.Contains(got, "value := 2") {
		t.Fatalf("안 바뀌었다:\n%s", got)
	}
	// 손대지 않은 줄의 전각 공백은 그대로 남아야 한다(#145).
	if !strings.Contains(got, "연결　보류") {
		t.Fatalf("원문의 전각 공백이 보통 공백으로 훼손됐다:\n%q", got)
	}
}
