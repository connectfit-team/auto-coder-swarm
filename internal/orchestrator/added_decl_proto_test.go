package orchestrator

import "testing"

// 계약에 필드·RPC 를 더한 것도 「새로 생긴 이름」 이다.
//
// .proto 모양을 하나도 못 알아봐서, 계약을 고치라고 넘긴 일이 시작하자마자
// 「새로 생긴 이름이 없다」 로 막혔다.
func TestAddedDeclarationsReadsProto(t *testing.T) {
	diff := `--- a/ceoweb/v1/connect.service.proto
+++ b/ceoweb/v1/connect.service.proto
@@
 service Internal {
+  rpc HoldRequest(RequestHoldRequest) returns (ResponseHoldRequest) {}
 }
+
+message RequestHoldRequest {
+  int64 request_id = 1;
+  bool hold = 2;
+}
+
+enum ConnectState {
+  CONNECT_STATE_HOLD = 3;
+}
`
	got := addedDeclarations(diff)
	if len(got) < 4 {
		t.Fatalf("계약에 더한 것을 못 알아봤다 — %d개: %v", len(got), got)
	}
	want := []string{"rpc HoldRequest", "message RequestHoldRequest", "bool hold = 2;", "enum ConnectState"}
	for _, w := range want {
		found := false
		for _, g := range got {
			if len(g) >= len(w) && g[:len(w)] == w {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%q 를 못 알아봤다: %v", w, got)
		}
	}
}

// .proto 패턴은 .proto 에만 쓴다. 다른 말에서 `x = 5;` 는 그냥 대입이다.
func TestProtoPatternsStayInProtoFiles(t *testing.T) {
	diff := `--- a/src/lib/thing.ts
+++ b/src/lib/thing.ts
@@
 function already() {
+  count = 5;
+  total = 7;
 }
`
	if got := addedDeclarations(diff); len(got) != 0 {
		t.Errorf("대입을 새 선언으로 셌다: %v", got)
	}
}

// 한 diff 안에 두 말이 섞여도 파일마다 맞는 패턴을 쓴다.
func TestDeclPatternsFollowTheFile(t *testing.T) {
	diff := `--- a/src/lib/thing.ts
+++ b/src/lib/thing.ts
@@
+  count = 5;
--- a/ceoweb/v1/connect.service.proto
+++ b/ceoweb/v1/connect.service.proto
@@
+  bool hold = 2;
`
	got := addedDeclarations(diff)
	if len(got) != 1 || got[0] != "bool hold = 2;" {
		t.Errorf("파일을 따라가지 않았다: %v", got)
	}
}
