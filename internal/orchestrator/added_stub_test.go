package orchestrator

import "testing"

// 「기능을 추가할거야」 에 빈 껍데기가 넘어왔다(W-58244). 이름은 생겼지만
// 속이 비어 있으니 아무 일도 하지 않는다.
func TestStubDoesNotCount(t *testing.T) {
	stub := `--- a/src/lib/i18n/index.ts
+++ b/src/lib/i18n/index.ts
@@
+
+export function useI18n() {
+    // Implementation of useI18n function
+}
`
	if bad := CheckNewFeatureAddedSomething(stub); len(bad) == 0 {
		t.Errorf("빈 껍데기를 통과시켰다 — 센 것 %v", addedDeclarations(stub))
	}

	// 속이 있으면 만든 것이다.
	real := `--- a/src/lib/i18n/index.ts
+++ b/src/lib/i18n/index.ts
@@
+export function useI18n() {
+    return getContext(I18N_KEY);
+}
`
	if bad := CheckNewFeatureAddedSomething(real); len(bad) != 0 {
		t.Errorf("속이 있는데 막았다: %v", bad)
	}

	// 열거 값 하나는 중괄호를 열지 않으므로 그대로 센다.
	enum := "--- a/x.ts\n+++ b/x.ts\n@@\n+    HOLD = 'hold',\n"
	if bad := CheckNewFeatureAddedSomething(enum); len(bad) != 0 {
		t.Errorf("열거 값을 막았다: %v", bad)
	}
}
