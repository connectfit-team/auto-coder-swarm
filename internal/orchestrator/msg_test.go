package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 계약 파일은 서비스와 메시지를 나눠 두는 일이 흔하다. 파일만 알려 주면
// 자식이 그 파일에서 타입을 못 찾고 없는 메시지를 지어낸다.
func TestProtoFileDeclaring(t *testing.T) {
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
	mk("ceoweb/v1/connect.service.proto", "syntax=\"proto3\";\nservice Internal {\n  rpc A(B) returns (C) {}\n}\n")
	mk("ceoweb/v1/connect.communication.proto", "syntax=\"proto3\";\nmessage ReceivedRequest {\n  string id = 1;\n}\n")

	if got := protoFileDeclaring(root, "ReceivedRequest"); got != "ceoweb/v1/connect.communication.proto" {
		t.Errorf("메시지가 있는 파일을 못 짚었다: %q", got)
	}
	if got := protoFileDeclaring(root, "NoSuchMessage"); got != "" {
		t.Errorf("없는 메시지를 찾았다고 했다: %q", got)
	}
	if got := protoFileDeclaring("", "ReceivedRequest"); got != "" {
		t.Errorf("경로가 없는데 찾았다: %q", got)
	}
}

// 넘길 때 붙일 메시지를 알려 주고, 새로 만들지 말라고 못박는다.
func TestHandoffNamesTheMessage(t *testing.T) {
	src := readSource(t, "state_chain.go")
	for _, must := range []string{
		"protoFileDeclaring(",
		"붙일 메시지",
		"같은 이름으로 새로 만들지 마라",
	} {
		if !strings.Contains(src, must) {
			t.Errorf("state_chain.go 에 %q 가 없다", must)
		}
	}
	if !strings.Contains(readSource(t, "owner_of_state.go"), "t.stateType = typeName") {
		t.Error("타입 이름을 기억하지 않는다 — 넘길 때 알려 줄 수 없다")
	}
}
