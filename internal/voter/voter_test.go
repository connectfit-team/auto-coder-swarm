package voter

import (
	"strings"
	"testing"
)

func TestConsensusKeyCollapsesWhitespace(t *testing.T) {
	a := consensusKey("REPO: cms\nFILE: a.ts\nHOW:  고친다")
	b := consensusKey("REPO: cms\n\nFILE: a.ts\nHOW: 고친다")
	if a != b {
		t.Errorf("띄어쓰기만 다른 답을 다르게 셌다:\n%q\n%q", a, b)
	}
}

func TestConsensusKeyDoesNotEatLineFormat(t *testing.T) {
	// HOW 안의 코드 예시에 중괄호가 있어도 줄 형식이 살아 있어야 한다.
	raw := "REPO: cms\nFILE: a.ts\nHOW: 이렇게 바꾼다\n```\nworkAt: {\n  lt: x\n}\n```\nEND"
	k := consensusKey(raw)
	if !strings.Contains(k, "FILE: a.ts") {
		t.Errorf("줄 형식이 열쇠에서 사라졌다: %q", k)
	}
}

func TestConsensusKeyUsesJSONWhenPresent(t *testing.T) {
	// 앞뒤에 잡담이 붙어도 JSON 이 같으면 같은 답이다.
	a := consensusKey("네, 계획입니다:\n{\"repo_name\":\"cms\"}\n감사합니다")
	b := consensusKey("{\"repo_name\":\"cms\"}")
	if a != b {
		t.Errorf("같은 JSON 을 다르게 셌다:\n%q\n%q", a, b)
	}
}
