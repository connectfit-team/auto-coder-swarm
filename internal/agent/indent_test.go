package agent

import (
	"strings"
	"testing"
)

func TestApplyEditBlocksKeepsIndent(t *testing.T) {
	// 실제로 무너졌던 모양 — workAt: { 안의 gte·lt 가 바깥 깊이로 나왔다.
	orig := "        where: {\n            workAt: {\n                gte: A,\n                lt: B\n            }\n        }\n"
	raw := "<<<<<<< SEARCH\nworkAt: {\n    gte: A,\n    lt: B\n}\n=======\nworkAt: {\n    gte: A,\n    lt: C\n}\n>>>>>>> REPLACE"

	got, err := applyEditBlocks(orig, raw)
	if err != nil {
		t.Fatalf("적용 실패: %v", err)
	}
	if !strings.Contains(got, "lt: C") {
		t.Fatalf("안 바뀌었다:\n%s", got)
	}
	// workAt 은 12칸, 그 안은 16칸이어야 한다.
	if !strings.Contains(got, "            workAt: {") {
		t.Errorf("바깥 들여쓰기가 무너졌다:\n%s", got)
	}
	if !strings.Contains(got, "                gte: A,") || !strings.Contains(got, "                lt: C") {
		t.Errorf("안쪽 들여쓰기가 무너졌다:\n%s", got)
	}
}

func TestReindent(t *testing.T) {
	got := reindent([]string{"a {", "    b", "}"}, "        ")
	want := []string{"        a {", "            b", "        }"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("줄 %d = %q, want %q", i, got[i], want[i])
		}
	}
	// 빈 줄에는 공백을 붙이지 않는다.
	if r := reindent([]string{"a", "", "b"}, "  "); r[1] != "" {
		t.Errorf("빈 줄에 공백을 붙였다: %q", r[1])
	}
}
