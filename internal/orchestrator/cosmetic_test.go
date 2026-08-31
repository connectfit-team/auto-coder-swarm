package orchestrator

import "testing"

func TestIsCosmeticDiff(t *testing.T) {
	// 실제로 올라왔던 diff — 파일 끝 개행 하나만 바뀌었다.
	newlineOnly := `diff --git a/src/lib/utils/workstamp.ts b/src/lib/utils/workstamp.ts
index f2555181..f3741b89 100644
--- a/src/lib/utils/workstamp.ts
+++ b/src/lib/utils/workstamp.ts
@@ -65,4 +65,4 @@ export function hexToRgba(hex: string, alpha: number): string {
 
     // Return rgba string
     return ` + "`rgba(${r}, ${g}, ${b}, ${a})`" + `;
-}
+}
\ No newline at end of file`
	if !isCosmeticDiff(newlineOnly) {
		t.Error("개행만 바뀐 diff 를 못 잡았다")
	}

	indentOnly := `--- a/a.ts
+++ b/a.ts
-  const a = 1;
+    const a = 1;`
	if !isCosmeticDiff(indentOnly) {
		t.Error("들여쓰기만 바뀐 diff 를 못 잡았다")
	}

	real := `--- a/a.ts
+++ b/a.ts
-  const end = new Date(y, m, 0);
+  const end = new Date(y, m + 1, 0);`
	if isCosmeticDiff(real) {
		t.Error("실제 수정을 공백 변경으로 봤다")
	}

	added := `--- a/a.ts
+++ b/a.ts
+  const end = endOfMonth(month);`
	if isCosmeticDiff(added) {
		t.Error("줄이 늘어난 것을 공백 변경으로 봤다")
	}
}
