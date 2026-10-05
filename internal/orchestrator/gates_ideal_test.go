package orchestrator

import (
	"fmt"
	"testing"
)

// **관문 전체가 이상적인 답을 통과시키는가.**
//
// #167 에서 내 관문 하나가 이상적인 답을 막고 있는 것을 찾았다. 판정기에
// 넣어 보고서야 알았다. 그 교훈을 관문 전체에 대고 한 번 더 한다 —
// 넘을 수 없는 관문은 관문이 아니다.
//
// 이 모양은 ACCEPTANCE.md 가 「이 모양을 기준으로 본다」 고 적어 둔 것이다.
func TestGatesLetTheIdealAnswerThrough(t *testing.T) {
	repo := "/home/cnf/cie-repos/proto-ceowebapis"
	ideal := `diff --git a/ceoweb/v1/connect.communication.proto b/ceoweb/v1/connect.communication.proto
--- a/ceoweb/v1/connect.communication.proto
+++ b/ceoweb/v1/connect.communication.proto
@@ -51,6 +51,14 @@ message ReceivedRequest {
   string existing_workplace_id = 14;
+
+  // 수락도 거절도 하지 않고 미뤄 둔 상태.
+  PendingStatus pending_status = 15;
+}
+
+enum PendingStatus {
+  PENDING_STATUS_UNSPECIFIED = 0;
+  PENDING_STATUS_PENDING = 1;
+  PENDING_STATUS_RELEASED = 2;
 }
diff --git a/ceoweb/v1/connect.service.proto b/ceoweb/v1/connect.service.proto
--- a/ceoweb/v1/connect.service.proto
+++ b/ceoweb/v1/connect.service.proto
@@ -39,4 +39,14 @@ service Internal {
   rpc RequestProfileUpdate(RequestProfileUpdateAlarm) returns (ResponseProfileUpdateAlarm) {}
+  rpc UpdateReceivedRequestStatus(RequestUpdateReceivedRequestStatus) returns (ResponseUpdateReceivedRequestStatus) {}
+}
+
+message RequestUpdateReceivedRequestStatus {
+  string request_id = 1;
+  PendingStatus pending_status = 2;
+}
+
+message ResponseUpdateReceivedRequestStatus {
+  bool updated = 1;
 }
`
	bad := CheckProtoChange(repo, ideal)
	for _, v := range bad {
		fmt.Println("  막음:", v.Why)
		for _, e := range v.Evidence {
			fmt.Println("        ", e)
		}
	}
	if len(bad) > 0 {
		t.Fatalf("관문이 이상적인 답을 %d개 이유로 막는다 — 넘을 수 없는 관문이다", len(bad))
	}

	if v := checkProcedureViolations(ideal); len(v) > 0 {
		for _, s := range v {
			fmt.Println("  절차 위반:", s)
		}
		t.Fatalf("절차 검사가 이상적인 답을 막는다: %d개", len(v))
	}
}
