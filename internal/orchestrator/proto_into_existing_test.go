package orchestrator

import (
	"strings"
	"testing"
)

// 실측 W-53071 의 수정 그대로다. 자리·이름·타입은 다 맞췄는데 **있는 메시지가
// 아니라 새로 만든 메시지**에 넣었다 — 아무도 안 쓰는 자리다.
const w53071 = `+++ b/ceoweb/v1/connect.service.proto
+  rpc UpdateConnectRequestStatus(RequestUpdateConnectRequestStatus) returns (ResponseUpdateConnectRequestStatus) {}
+}
+
+enum ConnectRequestStatus {
+  CONNECT_REQUEST_STATUS_UNSPECIFIED = 0;
+}
+
+message RequestConnect {
+  string request_id = 1;
+  ConnectRequestStatus status = 2;
+}
+
+message RequestUpdateConnectRequestStatus {
+  string request_id = 1;
+  ConnectRequestStatus status = 2;
+}
`

func TestStateWithNoHomeIsCaught(t *testing.T) {
	got := fieldWentIntoNewMessage(w53071, "ReceivedRequest")
	if len(got) != 1 {
		t.Fatalf("새 메시지에만 넣은 것을 안 물었다: %v", got)
	}
	all := got[0].Why + " " + strings.Join(got[0].Evidence, " ")
	for _, want := range []string{"ReceivedRequest", "RequestConnect", "아무도 받거나 돌려주지 않는다"} {
		if !strings.Contains(all, want) {
			t.Fatalf("%q 가 없다:\n%s", want, all)
		}
	}
}

// 있던 메시지에 넣었으면 통과한다 — 새 메시지를 함께 만들어도 괜찮다.
func TestFieldIntoExistingMessagePasses(t *testing.T) {
	diff := `+++ b/ceoweb/v1/connect.communication.proto
   string existing_workplace_id = 14;
+  PendingStatus pending_status = 15;
 }
+
+message RequestUpdateX {
+  string request_id = 1;
+}
`
	if got := fieldWentIntoNewMessage(diff, "ReceivedRequest"); len(got) != 0 {
		t.Fatalf("있던 메시지에 넣었는데 물었다: %v", got)
	}
}

// 어느 메시지인지 안 알려 줬으면 아무 말도 하지 않는다.
func TestNoStateTypeNoVerdict(t *testing.T) {
	if got := fieldWentIntoNewMessage(w53071, ""); len(got) != 0 {
		t.Fatalf("이름을 모르는데 단정한다: %v", got)
	}
}

// 필드를 아예 안 넣었으면 이 관문이 할 말은 없다 — 다른 관문의 일이다.
func TestNoFieldsNoVerdict(t *testing.T) {
	diff := "+++ b/a.proto\n+// 주석만 고쳤다\n"
	if got := fieldWentIntoNewMessage(diff, "ReceivedRequest"); len(got) != 0 {
		t.Fatalf("필드가 없는데 물었다: %v", got)
	}
}
