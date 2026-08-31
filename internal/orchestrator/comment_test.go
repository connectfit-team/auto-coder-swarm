package orchestrator

import "testing"

func TestIsCommentOnlyDiff(t *testing.T) {
	// 실제로 올라왔던 diff — 감사 로그 주석에 한 마디 덧붙인 것뿐이다.
	commentOnly := `--- a/+server.ts
+++ b/+server.ts
-    // 감사 로그: 근무 기록 XLSX 다운로드 (fire-and-forget)
+    // 감사 로그: 근무 기록 XLSX 다운로드 (fire-and-forget). : 이 방법을 따지세요.`
	if !isCommentOnlyDiff(commentOnly) {
		t.Error("주석만 바뀐 diff 를 못 잡았다")
	}

	real := `--- a/a.ts
+++ b/a.ts
-  lt: new Date(y, m + 1, 0)
+  lt: new Date(y, m + 1, 1)`
	if isCommentOnlyDiff(real) {
		t.Error("실제 수정을 주석 변경으로 봤다")
	}

	mixed := `--- a/a.ts
+++ b/a.ts
-  // 말일이 빠진다
-  lt: new Date(y, m + 1, 0)
+  // 말일까지 포함한다
+  lt: new Date(y, m + 1, 1)`
	if isCommentOnlyDiff(mixed) {
		t.Error("주석과 코드가 함께 바뀐 것을 주석만으로 봤다")
	}

	if isCommentOnlyDiff("") {
		t.Error("빈 diff 를 주석 변경으로 봤다")
	}
}

func TestWantsCommentWork(t *testing.T) {
	if !wantsCommentWork("이 파일에 주석을 달아라") {
		t.Error("주석 작업을 못 알아봤다")
	}
	if wantsCommentWork("월별 근무 조회에서 말일이 빠진다. 고쳐라") {
		t.Error("버그 수정을 주석 작업으로 봤다")
	}
}
