package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleService = `syntax = "proto3";

service Internal {
  // 보낸 초대 목록
  rpc ListSentInvites(RequestListSentInvites) returns (ResponseListSentInvites) {
    option (x) = 1;
  }
  rpc ListReceivedRequests(RequestListReceivedRequests) returns (ResponseListReceivedRequests) {}
  rpc CreateInvites(RequestCreateInvites) returns (ResponseCreateInvites) {}
}
`

func writeService(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "ceoweb/v1"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ceoweb/v1/connect.service.proto"),
		[]byte(sampleService), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// rpc 본문의 닫는 괄호에서 끊기면 rpc 를 하나밖에 못 보여 준다.
func TestServiceExampleSurvivesRPCBodies(t *testing.T) {
	got := protoServiceExample(writeService(t), "ceoweb/v1/connect.service.proto")
	for _, want := range []string{"service Internal {", "ListSentInvites", "ListReceivedRequests", "그 안에"} {
		if !strings.Contains(got, want) {
			t.Fatalf("%q 가 없다:\n%s", want, got)
		}
	}
	// 둘까지만.
	if strings.Contains(got, "CreateInvites") {
		t.Fatalf("두 개만 보여야 한다:\n%s", got)
	}
	// 본문은 안 끌고 온다.
	if strings.Contains(got, "option (x)") {
		t.Fatalf("rpc 본문까지 끌고 왔다:\n%s", got)
	}
}

func TestServiceExampleEmptyWithoutService(t *testing.T) {
	dir := writeService(t)
	if err := os.WriteFile(filepath.Join(dir, "ceoweb/v1/c.proto"),
		[]byte("message A { string b = 1; }"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := protoServiceExample(dir, "ceoweb/v1/c.proto"); got != "" {
		t.Fatalf("service 가 없는 파일인데 내놓는다:\n%s", got)
	}
}
