package agent

import (
	"strings"
	"testing"
)

// 통째로 다시 쓰다 잘린 파일을 쓰기 전에 잡는다. 빌드는 계약을 안 본다.
func TestProtoSyntaxCatchesTruncation(t *testing.T) {
	cut := `syntax = "proto3";
package p;

message ReceivedRequest {
  string id = 1;
`
	err := CheckSyntax("a.proto", cut)
	if err == nil {
		t.Fatal("잘린 파일을 통과시켰다")
	}
	if !strings.Contains(err.Error(), "닫히지 않았다") {
		t.Errorf("까닭이 틀렸다: %v", err)
	}
}

func TestProtoSyntaxCatchesBadShapes(t *testing.T) {
	cases := map[string]string{
		"이름 없는 message": "message {\n  string a = 1;\n}\n",
		"여는 괄호 없음":      "message Foo\n  string a = 1;\n}\n",
		"열지 않은 }":       "message Foo {\n}\n}\n",
		"필드 번호 0":       "message Foo {\n  string a = 0;\n}\n",
		"proto 가 쓰는 번호": "message Foo {\n  string a = 19001;\n}\n",
	}
	for name, src := range cases {
		if err := CheckSyntax("a.proto", src); err == nil {
			t.Errorf("%s: 통과시켰다", name)
		}
	}
}

// 멀쩡한 계약을 막으면 안 된다.
func TestProtoSyntaxPassesRealShapes(t *testing.T) {
	ok := `syntax = "proto3";
package ceo.connect.ceoweb;

import "ceowebapis/ceoweb/v1/connect.communication.proto";

// 주석 { 안의 괄호는 세지 않는다 }
/* 여러 줄
   주석 { 도 */
message ReceivedRequest {
  string id = 1;
  repeated string tags = 2;
  ConnectHold hold = 15 [deprecated = true];

  enum Nested {
    NESTED_UNSPECIFIED = 0;
  }
  oneof kind {
    string a = 16;
    int32 b = 17;
  }
}

service Internal {
  rpc List(RequestList) returns (ResponseList) {
    option idempotency_level = NO_SIDE_EFFECTS;
  }
}
`
	if err := CheckSyntax("a.proto", ok); err != nil {
		t.Errorf("멀쩡한 계약을 막았다: %v", err)
	}
}

// enum 값의 0 은 오히려 있어야 하는 것이다. 필드 규칙을 거기 적용하면 안 된다.
func TestProtoSyntaxAllowsEnumZero(t *testing.T) {
	if err := CheckSyntax("a.proto", "enum S {\n  S_UNSPECIFIED = 0;\n  S_ONE = 1;\n}\n"); err != nil {
		t.Errorf("enum 의 0 을 막았다: %v", err)
	}
	// 그래도 필드의 0 은 막는다
	if err := CheckSyntax("a.proto", "message M {\n  string a = 0;\n}\n"); err == nil {
		t.Error("필드 번호 0 을 통과시켰다")
	}
}
