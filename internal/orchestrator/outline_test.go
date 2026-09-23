package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 모델이 뒤져서 알아낼 일이 아니라 세어서 주면 되는 것이다.
func TestProtoOutlineShowsWhatExists(t *testing.T) {
	root := t.TempDir()
	mk := func(rel, body string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("ceoweb/v1/connect.service.proto", `service Internal {
  rpc ListSentInvites(RequestListSentInvites) returns (ResponseListSentInvites) {}
  rpc AcceptRequest(RequestAcceptRequest) returns (ResponseAcceptRequest) {}
}
`)
	mk("ceoweb/v1/connect.communication.proto", `message ReceivedRequest {}
message RequestListSentInvites {}
enum Foo {}
`)
	mk("other/v1/x.proto", "service Elsewhere {}\n")

	got := protoOutline(root, "ceoweb/v1/connect.communication.proto", "ceoweb/v1/connect.service.proto")
	for _, must := range []string{
		"이미 있는 것",
		"service Internal {",
		"ListSentInvites",
		"AcceptRequest",
		"message ReceivedRequest",
		"enum Foo",
	} {
		if !strings.Contains(got, must) {
			t.Errorf("간추린 글에 %q 가 없다:\n%s", must, got)
		}
	}
	if strings.Contains(got, "Elsewhere") {
		t.Errorf("다른 폴더까지 실었다:\n%s", got)
	}
	// 고칠 파일이 먼저 나와야 한다.
	iMsg := strings.Index(got, "connect.communication.proto")
	iSvc := strings.Index(got, "connect.service.proto")
	if iMsg < 0 || iSvc < 0 {
		t.Fatalf("두 파일이 다 없다:\n%s", got)
	}
	if got := protoOutline(""); got != "" {
		t.Errorf("경로가 없는데 지어냈다: %q", got)
	}
}

// 넘기는 쪽지에 실려야 한다.
func TestHandoffCarriesTheOutline(t *testing.T) {
	if !strings.Contains(readSource(t, "state_chain.go"), "protoOutline(") {
		t.Error("넘길 때 이미 있는 것을 보여 주지 않는다")
	}
}
