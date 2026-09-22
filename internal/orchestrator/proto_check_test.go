package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func protoRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// 이미 있는 메시지를 다른 파일에 다시 정의하면 protoc 이 깨진다.
// 생성물을 다시 만들지 않으므로 go build 는 통과한다 — 실측으로 그 수정이
// 승인 대기까지 갔다.
func TestProtoCheckCatchesDuplicateMessage(t *testing.T) {
	root := protoRepo(t, map[string]string{
		"ceoweb/v1/connect.communication.proto": `syntax = "proto3";
package ceo.connect.ceoweb;

message ReceivedRequest {
  string id = 1;
  int32 existing_connection = 13;
}
`,
		"ceoweb/v1/connect.service.proto": `syntax = "proto3";
package ceo.connect.ceoweb;

message ReceivedRequest {
  string request_id = 1;
}
`,
	})
	diff := `--- a/ceoweb/v1/connect.service.proto
+++ b/ceoweb/v1/connect.service.proto
@@
+message ReceivedRequest {
+  string request_id = 1;
+}
`
	bad := CheckProtoChange(root, diff)
	if len(bad) == 0 {
		t.Fatal("중복 정의를 못 잡았다")
	}
	if !strings.Contains(bad[0].Why, "ReceivedRequest") {
		t.Errorf("무엇이 겹치는지 안 적었다: %s", bad[0].Why)
	}
}

// 한 메시지 안에서 번호가 겹치면 옛 데이터가 다른 필드로 읽힌다.
func TestProtoCheckCatchesDuplicateFieldNumber(t *testing.T) {
	root := protoRepo(t, map[string]string{
		"a.proto": `syntax = "proto3";
package p;

message M {
  string a = 1;
  string b = 1;
}
`,
	})
	diff := "--- a/a.proto\n+++ b/a.proto\n@@\n+  string b = 1;\n"
	bad := CheckProtoChange(root, diff)
	if len(bad) == 0 {
		t.Fatal("겹치는 번호를 못 잡았다")
	}
	if !strings.Contains(bad[0].Why, "1") {
		t.Errorf("어느 번호인지 안 적었다: %s", bad[0].Why)
	}
}

// 계약은 더하기만 한다. 있던 필드·RPC 를 지우면 쓰는 쪽이 깨진다.
func TestProtoCheckCatchesRemovedField(t *testing.T) {
	root := protoRepo(t, map[string]string{"a.proto": "syntax = \"proto3\";\npackage p;\nmessage M { string a = 1; }\n"})
	diff := `--- a/a.proto
+++ b/a.proto
@@
-  string existing_workplace_id = 14;
-  rpc OldOne(Req) returns (Res) {}
+  string renamed = 14;
`
	bad := CheckProtoChange(root, diff)
	found := false
	for _, b := range bad {
		if strings.Contains(b.Why, "있던 필드") {
			found = true
		}
	}
	if !found {
		t.Errorf("지운 필드·RPC 를 못 잡았다: %v", bad)
	}
}

// 바르게 더한 수정은 통과한다.
func TestProtoCheckPassesACleanAddition(t *testing.T) {
	root := protoRepo(t, map[string]string{
		"ceoweb/v1/connect.communication.proto": `syntax = "proto3";
package ceo.connect.ceoweb;

message ReceivedRequest {
  string id = 1;
  int32 existing_connection = 13;
  string existing_workplace_id = 14;
  ConnectHold hold = 15;
}

message ConnectHold {
  bool held = 1;
}
`,
	})
	diff := `--- a/ceoweb/v1/connect.communication.proto
+++ b/ceoweb/v1/connect.communication.proto
@@
+  ConnectHold hold = 15;
+
+message ConnectHold {
+  bool held = 1;
+}
`
	if bad := CheckProtoChange(root, diff); len(bad) > 0 {
		t.Errorf("멀쩡한 수정을 막았다: %v", bad)
	}
}

// .proto 를 안 고친 수정에는 끼어들지 않는다.
func TestProtoCheckIgnoresNonProtoDiffs(t *testing.T) {
	if bad := CheckProtoChange(t.TempDir(), "--- a/x.ts\n+++ b/x.ts\n@@\n+const a = 1;\n"); len(bad) > 0 {
		t.Errorf("계약이 아닌 수정에 끼어들었다: %v", bad)
	}
}

// 관문이 달려 있어야 한다.
func TestProtoGateIsWired(t *testing.T) {
	src := readSource(t, "handover.go")
	if !strings.Contains(src, "CheckProtoChange(t.repoPath, diff)") {
		t.Error("계약 관문이 handOver 에 없다")
	}
}
