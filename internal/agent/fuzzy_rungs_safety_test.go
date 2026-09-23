package agent

import (
	"os"
	"strings"
	"testing"
)

// 아랫단이 **없는 것을 찾았다고 하면** 안 된다. 그것이 이 단들의 유일한 위험이다.
func TestApproxDoesNotInventMatches(t *testing.T) {
	src, err := os.ReadFile("edit_blocks.go")
	if err != nil {
		t.Skip("소스를 못 읽었다")
	}
	lines := strings.Split(string(src), "\n")

	absent := []string{
		"func 이런것은없다() {",
		"\treturn 완전히다른것",
		"}",
	}
	if at := blockAnchorAt(lines, absent); len(at) != 0 {
		t.Fatalf("없는 것을 앵커로 찾았다: %v", at)
	}
	if at := contextAwareAt(lines, absent); len(at) != 0 {
		t.Fatalf("없는 것을 줄별 닮은 정도로 찾았다: %v", at)
	}

	// 한 줄짜리는 앵커가 성립하지 않는다 — 우연히 맞을 자리가 너무 많다.
	if at := blockAnchorAt(lines, []string{"}"}); len(at) != 0 {
		t.Fatalf("한 줄에 앵커가 걸렸다: %v", at)
	}
}

// 실제 소스에서 진짜 있는 블록은 정확한 단이 먼저 잡아야 한다 —
// 아랫단까지 내려가면 안 된다.
func TestRealBlockTakenByExactRung(t *testing.T) {
	src, err := os.ReadFile("window.go")
	if err != nil {
		t.Skip("소스를 못 읽었다")
	}
	body := string(src)
	i := strings.Index(body, "func InputTokenBudget() int {")
	if i < 0 {
		t.Skip("기준 블록이 사라졌다")
	}
	block := body[i : i+strings.Index(body[i:], "\n}\n")+3]

	LastApproxRung()
	raw := "<<<<<<< SEARCH\n" + block + "=======\n" + block + "// 덧붙임\n>>>>>>> REPLACE"
	if _, err := applyEditBlocks(body, raw); err != nil {
		t.Fatalf("진짜 있는 블록을 못 찾았다: %v", err)
	}
	if r := LastApproxRung(); r != "" {
		t.Fatalf("정확히 있는 블록인데 아랫단까지 내려갔다: %s", r)
	}
}
