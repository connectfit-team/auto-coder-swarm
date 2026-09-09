package orchestrator

import (
	"strings"
	"testing"
)

// 코더가 "고치기 전" 코드를 지어내지 않게 하는 조건.
func TestCoderNewFeatureHint(t *testing.T) {
	h := CoderNewFeatureHint()
	for _, must := range []string{
		"지어내지 마라",       // 핵심
		"원문에 실제로 있는 줄만", // SEARCH 규칙
		"REPLACE 는 SEARCH 를 품는다", // 더하는 방법
		"온전한 선언",        // 조각 금지
	} {
		if !strings.Contains(h, must) {
			t.Errorf("조건문에 %q 가 없다:\n%s", must, h)
		}
	}
}
