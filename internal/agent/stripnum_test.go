package agent

import (
	"strings"
	"testing"
)

func TestStripLineNumbers(t *testing.T) {
	in := "30:             lt: new Date(y, m + 1, 0)\n31:         }"
	got := stripLineNumbers(in)
	if strings.Contains(got, "30:") || !strings.Contains(got, "lt: new Date") {
		t.Errorf("번호를 못 뗐다: %q", got)
	}
	// 코드 안의 라벨을 번호로 오인하면 안 된다.
	code := "switch x {\ncase 1:\n\tdoThing()\n}"
	if stripLineNumbers(code) != code {
		t.Errorf("코드를 건드렸다: %q", stripLineNumbers(code))
	}
	// 일부 줄에만 번호가 있으면 손대지 않는다.
	mixed := "1: a\nb"
	if stripLineNumbers(mixed) != mixed {
		t.Errorf("일부만 번호인데 뗐다: %q", stripLineNumbers(mixed))
	}
}

func TestApplyEditBlocksWithLineNumbers(t *testing.T) {
	orig := "const a = 1;\n            lt: new Date(y, m + 1, 0)\nconst b = 2;\n"
	raw := "<<<<<<< SEARCH\n30:             lt: new Date(y, m + 1, 0)\n=======\n30:             lt: new Date(y, m + 1, 1)\n>>>>>>> REPLACE"
	got, err := applyEditBlocks(orig, raw)
	if err != nil {
		t.Fatalf("줄 번호가 붙은 블록을 못 썼다: %v", err)
	}
	if !strings.Contains(got, "m + 1, 1") || strings.Contains(got, "30:") {
		t.Errorf("잘못 적용됐다:\n%s", got)
	}
}
