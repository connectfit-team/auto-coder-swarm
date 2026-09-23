package orchestrator

import (
	"strings"
	"testing"
)

// 실측 W-13896 의 수정 그대로다.
const realEmptyDiff = `diff --git a/ceoweb/v1/connect.service.proto b/ceoweb/v1/connect.service.proto
--- a/ceoweb/v1/connect.service.proto
+++ b/ceoweb/v1/connect.service.proto
@@ -39,4 +39,17 @@ service Internal {
+  rpc UpdateReceivedRequest(RequestUpdateReceivedRequest) returns (ResponseUpdateReceivedRequest) {}
+}
+
+// RequestUpdateReceivedRequest 메시지 정의
+message RequestUpdateReceivedRequest {
+  // 필요한 필드 추가
+}
+
+// ResponseUpdateReceivedRequest 메시지 정의
+message ResponseUpdateReceivedRequest {
+  // 필요한 필드 추가
 }
`

func TestEmptyNewMessageIsCaught(t *testing.T) {
	got := emptyNewMessage(realEmptyDiff)
	if len(got) != 2 {
		t.Fatalf("빈 메시지 둘을 다 잡아야 한다: %d개 %v", len(got), got)
	}
	all := ""
	for _, v := range got {
		all += v.Why + " " + strings.Join(v.Evidence, " ")
	}
	for _, want := range []string{"RequestUpdateReceivedRequest", "ResponseUpdateReceivedRequest", "필드가 하나도 없다", "자리표시"} {
		if !strings.Contains(all, want) {
			t.Fatalf("%q 가 없다:\n%s", want, all)
		}
	}
}

// 한 줄로 닫은 것은 「받을 것이 없다」 는 뜻이다 — 이 계약에 실제로 있다.
func TestOneLineEmptyMessageIsAllowed(t *testing.T) {
	diff := "+++ b/a.proto\n+message RequestListSentInvites {}\n"
	if got := emptyNewMessage(diff); len(got) != 0 {
		t.Fatalf("한 줄로 닫은 것을 물었다: %v", got)
	}
}

func TestFilledMessagePasses(t *testing.T) {
	diff := "+++ b/a.proto\n+message RequestX {\n+  // 대상 요청\n+  string request_id = 1;\n+}\n"
	if got := emptyNewMessage(diff); len(got) != 0 {
		t.Fatalf("속이 있는 메시지를 물었다: %v", got)
	}
	// repeated·optional 도 필드다.
	diff2 := "+++ b/a.proto\n+message RequestY {\n+  repeated InviteInput invites = 1;\n+}\n"
	if got := emptyNewMessage(diff2); len(got) != 0 {
		t.Fatalf("repeated 필드를 못 알아본다: %v", got)
	}
}

// 고치지 않은 줄(문맥)에 있는 빈 메시지를 물면 안 된다.
func TestUntouchedMessageIgnored(t *testing.T) {
	diff := "+++ b/a.proto\n message OldEmpty {\n   // 예전부터 이랬다\n }\n+message RequestZ {\n+  string id = 1;\n+}\n"
	if got := emptyNewMessage(diff); len(got) != 0 {
		t.Fatalf("건드리지 않은 것을 물었다: %v", got)
	}
}
