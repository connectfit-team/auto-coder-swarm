package orchestrator

import (
	"strings"
	"testing"
)

// 쌍대 비교 물음에 **근거 글이 들어가면 안 된다.**
//
// 근거를 붙여 재 보니 4쌍 중 2쌍이 틀렸고, 관련성이 아니라 덧붙은 말이
// 있느냐·긴가에 반응했다(경고가 적힌 계약을 골랐다). 경로만 주면 8/8 이다.
// 이 시험은 그 프롬프트가 다시 근거를 싣지 않게 못박는다.
func TestMatchPromptCarriesPathsOnly(t *testing.T) {
	src := readSource(t, "tournament.go")

	// 프롬프트를 만드는 자리에 evidence 가 다시 들어오면 안 된다.
	i := strings.Index(src, "func (t *taskContext) askMatch")
	if i < 0 {
		t.Fatal("askMatch 를 못 찾았다 — 이 시험이 지키려던 자리가 사라졌다")
	}
	body := src[i:]
	if j := strings.Index(body, "\n}\n"); j > 0 {
		body = body[:j]
	}
	if strings.Contains(body, "evidence") {
		t.Fatalf("쌍대 비교 물음에 근거 글이 실린다:\n%s", body)
	}
	for _, want := range []string{"[계약 A]", "[계약 B]", "A 또는 B 한 글자만"} {
		if !strings.Contains(body, want) {
			t.Fatalf("%q 가 없다:\n%s", want, body)
		}
	}
}
