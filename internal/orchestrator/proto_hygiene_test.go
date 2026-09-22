package orchestrator

import (
	"strings"
	"testing"
)

// 있는 service 에 rpc 를 더해야 한다. 새 service 를 만들면 쓰는 쪽이 부르지 않는다.
func TestProtoCheckCatchesNewServiceBesideOne(t *testing.T) {
	root := protoRepo(t, map[string]string{
		"a.proto": `syntax = "proto3";
package p;

service Internal {
  rpc Old(A) returns (B) {}
}

service ConnectService {
  rpc New(C) returns (D) {}
}
`,
	})
	diff := "--- a/a.proto\n+++ b/a.proto\n@@\n+service ConnectService {\n+  rpc New(C) returns (D) {}\n+}\n"
	bad := CheckProtoChange(root, diff)
	found := false
	for _, b := range bad {
		if strings.Contains(b.Why, "ConnectService") && strings.Contains(b.Why, "Internal") {
			found = true
		}
	}
	if !found {
		t.Errorf("새 service 를 못 잡았다: %v", bad)
	}
}

// 번호는 빈 다음 것을 쓴다. 멀리 뛰면 사이가 통째로 막힌다.
func TestProtoCheckCatchesFieldNumberGap(t *testing.T) {
	root := protoRepo(t, map[string]string{
		"a.proto": `syntax = "proto3";
package p;

message M {
  string a = 13;
  string b = 14;
  string c = 99;
}
`,
	})
	diff := "--- a/a.proto\n+++ b/a.proto\n@@\n+  string c = 99;\n"
	bad := CheckProtoChange(root, diff)
	found := false
	for _, b := range bad {
		if strings.Contains(b.Why, "99") && strings.Contains(b.Why, "15") {
			found = true
		}
	}
	if !found {
		t.Errorf("멀리 뛴 번호를 못 잡았다: %v", bad)
	}
	// 바로 다음 번호는 막지 않는다.
	ok := protoRepo(t, map[string]string{
		"a.proto": "syntax = \"proto3\";\npackage p;\nmessage M {\n  string a = 14;\n  string b = 15;\n}\n",
	})
	if bad := CheckProtoChange(ok, "--- a/a.proto\n+++ b/a.proto\n@@\n+  string b = 15;\n"); len(bad) > 0 {
		t.Errorf("바로 다음 번호를 막았다: %v", bad)
	}
}

// proto3 에서 0 은 값이 없을 때의 기본값이다. 뜻을 가진 값을 0 에 두면
// 여태 있던 것이 모두 그 값으로 읽힌다.
func TestProtoCheckCatchesMeaningfulEnumZero(t *testing.T) {
	root := protoRepo(t, map[string]string{"a.proto": "syntax = \"proto3\";\npackage p;\n"})
	bad := CheckProtoChange(root, `--- a/a.proto
+++ b/a.proto
@@
+enum PendingState {
+  PENDING = 0;
+  APPROVED = 1;
+}
`)
	found := false
	for _, b := range bad {
		if strings.Contains(b.Why, "0 번") {
			found = true
		}
	}
	if !found {
		t.Errorf("뜻을 가진 0 번을 못 잡았다: %v", bad)
	}
	// UNSPECIFIED 면 막지 않는다.
	if bad := CheckProtoChange(root, `--- a/a.proto
+++ b/a.proto
@@
+enum PendingState {
+  PENDING_STATUS_UNSPECIFIED = 0;
+  PENDING_STATUS_PENDING = 1;
+}
`); len(bad) > 0 {
		t.Errorf("바른 enum 을 막았다: %v", bad)
	}
}

// 같은 것을 한쪽은 만든 타입으로, 한쪽은 글자로 다루면 쓰는 쪽이 손으로 맞춰야 한다.
func TestProtoCheckCatchesSameNameDifferentType(t *testing.T) {
	root := protoRepo(t, map[string]string{"a.proto": "syntax = \"proto3\";\npackage p;\n"})
	bad := CheckProtoChange(root, `--- a/a.proto
+++ b/a.proto
@@
+  RequestPendingStatus pending_status = 15;
+  string new_status = 2;
`)
	found := false
	for _, b := range bad {
		if strings.Contains(b.Why, "_status") {
			found = true
		}
	}
	if !found {
		t.Errorf("한 수정 안의 타입 어긋남을 못 잡았다: %v", bad)
	}
	// 둘 다 글자면 짚지 않는다 — 만든 타입이 섞였을 때만 본다.
	if bad := CheckProtoChange(root, "--- a/a.proto\n+++ b/a.proto\n@@\n+  string a_status = 1;\n+  string b_status = 2;\n"); len(bad) > 0 {
		t.Errorf("글자끼리인데 막았다: %v", bad)
	}
}
