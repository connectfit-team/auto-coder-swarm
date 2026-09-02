package orchestrator

import (
	"strings"
	"testing"
)

func TestCommitMessageFor(t *testing.T) {
	got := commitMessageFor("cms 의 월별 근무 조회에서 매달 말일 데이터가 통째로 빠진다. 예를 들어 8월 조회에 8월 31일 기록이 하나도 안 나온다.")
	if !strings.HasPrefix(got, "fix: ") {
		t.Errorf("접두사가 없다: %q", got)
	}
	if !strings.Contains(got, "말일") {
		t.Errorf("무엇을 고쳤는지 안 남았다: %q", got)
	}
	if len([]rune(got)) > 70 {
		t.Errorf("너무 길다(%d자): %q", len([]rune(got)), got)
	}
	// 두 번째 문장은 안 들어와야 한다.
	if strings.Contains(got, "8월 31일 기록") {
		t.Errorf("첫 문장만 써야 한다: %q", got)
	}
	if got := commitMessageFor("   "); got != "fix: 자동 수정" {
		t.Errorf("빈 요청 폴백이 없다: %q", got)
	}
}
