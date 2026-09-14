package orchestrator

import "testing"

func TestCheckNewFeatureAddedSomething(t *testing.T) {
	// W-16333 — 있던 함수를 try/catch 로 감싸고 끝 줄바꿈을 지운 것이 전부.
	nothing := `--- a/src/lib/server/data/connection.ts
+++ b/src/lib/server/data/connection.ts
@@
-    const res = await getStaffClient().getStaffConnection({
-        workPlaceId
-    });
-    const info = res.connection;
-    if (!info) return { state: 'none' };
+    try {
+        const res = await getStaffClient().getStaffConnection({
+            workPlaceId
+        });
+        const info = res.connection;
+        if (!info) return { state: 'none' };
+    } catch (error) {
+        console.error("Error fetching connection info:", error);
+        return { state: 'error' };
+    }
+}
`
	if bad := CheckNewFeatureAddedSomething(nothing); len(bad) == 0 {
		t.Error("새로 생긴 이름이 없는데 통과시켰다")
	}

	// 열거에 값을 하나 더한 것은 만든 것이다.
	enumAdd := `--- a/src/lib/types/connectactions.ts
+++ b/src/lib/types/connectactions.ts
@@
+    HOLD = 'hold',
`
	if bad := CheckNewFeatureAddedSomething(enumAdd); len(bad) != 0 {
		t.Errorf("새 값을 더했는데 막았다: %v", bad)
	}

	// 새 함수도 마찬가지다.
	fnAdd := `--- a/src/lib/server/data/connectcud.ts
+++ b/src/lib/server/data/connectcud.ts
@@
+export async function holdInvite(session: Session, id: string) {
+    return { ok: true };
+}
`
	if bad := CheckNewFeatureAddedSomething(fnAdd); len(bad) != 0 {
		t.Errorf("새 함수를 더했는데 막았다: %v", bad)
	}

	// 주석만 더한 것은 이름이 아니다.
	commentOnly := `--- a/a.ts
+++ b/a.ts
@@
+// 여기에 보류를 더할 예정이다
`
	if bad := CheckNewFeatureAddedSomething(commentOnly); len(bad) == 0 {
		t.Error("주석만 더했는데 통과시켰다")
	}

	// Go 도 본다.
	goAdd := "--- a/x.go\n+++ b/x.go\n@@\n+func HoldConnect(id string) error { return nil }\n"
	if bad := CheckNewFeatureAddedSomething(goAdd); len(bad) != 0 {
		t.Errorf("Go 새 함수를 막았다: %v", bad)
	}
}
