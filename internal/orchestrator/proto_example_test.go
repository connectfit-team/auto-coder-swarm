package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleProto = `syntax = "proto3";

message ReceivedRequest {
  string id = 1;
}

message RequestAcceptRequest {
  string connect_id = 1;
  string request_id = 2;
}

message ResponseAcceptRequest {
  bool accepted = 1;
}

message RequestDeleteInvite {
  string invite_id = 1;
}

message ResponseDeleteInvite {
  bool deleted = 1;
}
`

func writeProto(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "ceoweb/v1"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ceoweb/v1/connect.communication.proto"),
		[]byte(sampleProto), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// 이름 규칙만으로는 모자랐다 — 실측에서 응답 모양을 지어냈다.
func TestProtoExampleShowsRealPairs(t *testing.T) {
	got := protoConventionExample(writeProto(t), "ceoweb/v1/connect.communication.proto", 2)
	if got == "" {
		t.Fatal("짝을 하나도 못 찾았다")
	}
	for _, want := range []string{"bool accepted = 1;", "string request_id = 2;", "지어내지 마라"} {
		if !strings.Contains(got, want) {
			t.Fatalf("%q 가 없다:\n%s", want, got)
		}
	}
	// 짝이 아닌 메시지는 끌어오지 않는다.
	if strings.Contains(got, "message ReceivedRequest") {
		t.Fatalf("짝이 아닌 것까지 넣었다:\n%s", got)
	}
}

func TestProtoExampleRespectsLimit(t *testing.T) {
	got := protoConventionExample(writeProto(t), "ceoweb/v1/connect.communication.proto", 1)
	if n := strings.Count(got, "message Request"); n != 1 {
		t.Fatalf("한 짝만 달라고 했는데 %d짝이다:\n%s", n, got)
	}
}

func TestProtoExampleOnlyForProto(t *testing.T) {
	dir := writeProto(t)
	if got := protoConventionExample(dir, "src/lib/connect.ts", 2); got != "" {
		t.Fatalf(".proto 가 아닌데 내놓는다:\n%s", got)
	}
	if got := protoConventionExample(dir, "ceoweb/v1/없다.proto", 2); got != "" {
		t.Fatalf("없는 파일인데 내놓는다:\n%s", got)
	}
}
