package orchestrator

import "testing"

func TestKeepRejectedDiff(t *testing.T) {
	tc := &taskContext{}

	// 반려 직전의 고침을 남긴다.
	tc.finalDiff = "diff --git a/x.ts b/x.ts\n-lt\n+lte\n"
	tc.keepRejectedDiff()
	if tc.lastRejectedDiff != tc.finalDiff {
		t.Error("되돌리기 전에 고침을 안 남겼다")
	}

	// 빈 diff 로는 덮어쓰지 않는다 — 되돌린 뒤에 불려도 남은 것이 살아야 한다.
	kept := tc.lastRejectedDiff
	tc.finalDiff = "   \n"
	tc.keepRejectedDiff()
	if tc.lastRejectedDiff != kept {
		t.Error("빈 diff 가 남아 있던 고침을 지웠다")
	}
}
