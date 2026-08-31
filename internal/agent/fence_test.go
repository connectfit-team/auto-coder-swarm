package agent

import (
	"strings"
	"testing"
)

func TestStripCodeFence(t *testing.T) {
	in := "```typescript\nworkAt: { lt: x }\n```"
	got := stripCodeFence(in)
	if strings.Contains(got, "```") || !strings.Contains(got, "workAt") {
		t.Errorf("울타리를 못 뗐다: %q", got)
	}
	// 울타리가 없으면 손대지 않는다.
	plain := "const a = 1;"
	if stripCodeFence(plain) != plain {
		t.Errorf("멀쩡한 코드를 건드렸다: %q", stripCodeFence(plain))
	}
	// 코드 안의 백틱 문자열은 울타리가 아니다.
	tick := "const s = `hello`;"
	if stripCodeFence(tick) != tick {
		t.Errorf("백틱 문자열을 지웠다: %q", stripCodeFence(tick))
	}
}

func TestApplyEditBlocksWithFence(t *testing.T) {
	orig := "const a = 1;\n        workAt: {\n            lt: new Date(y, m + 1, 0)\n        }\nconst b = 2;\n"
	raw := "<<<<<<< SEARCH\n```typescript\nworkAt: {\n    lt: new Date(y, m + 1, 0)\n}\n```\n" +
		"=======\n```typescript\nworkAt: {\n    lt: new Date(y, m + 1, 1)\n}\n```\n>>>>>>> REPLACE"
	got, err := applyEditBlocks(orig, raw)
	if err != nil {
		t.Fatalf("울타리가 붙은 블록을 못 썼다: %v", err)
	}
	if !strings.Contains(got, "m + 1, 1") || strings.Contains(got, "```") {
		t.Errorf("잘못 적용됐다:\n%s", got)
	}
	if !strings.Contains(got, "const b = 2;") {
		t.Errorf("나머지가 사라졌다:\n%s", got)
	}
}
