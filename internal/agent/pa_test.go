package agent

import (
	"strings"
	"testing"
)

// 계약(.proto)은 export 가 없다. TS·Go 모양만 보면 통째로 다시 쓰다가
// 메시지를 통째로 잃어도 아무도 모른다.
func TestLostNamesInProto(t *testing.T) {
	before := `syntax = "proto3";
message ReceivedRequest { string id = 1; }
message RequestListSentInvites {}
enum State { STATE_UNSPECIFIED = 0; }
service Internal {
  rpc ListSentInvites(A) returns (B) {}
}
`
	// 메시지 하나와 rpc 하나를 잃은 채 다시 쓴 것
	after := `syntax = "proto3";
message ReceivedRequest { string id = 1; }
enum State { STATE_UNSPECIFIED = 0; }
service Internal {
}
`
	lost := lostExportsFor("ceoweb/v1/a.proto", before, after)
	if len(lost) != 2 {
		t.Fatalf("잃은 이름을 못 잡았다: %v", lost)
	}
	joined := strings.Join(lost, ",")
	for _, want := range []string{"RequestListSentInvites", "ListSentInvites"} {
		if !strings.Contains(joined, want) {
			t.Errorf("%s 를 못 잡았다: %v", want, lost)
		}
	}
	// 그대로면 아무것도 잃지 않았다
	if lost := lostExportsFor("a.proto", before, before); len(lost) > 0 {
		t.Errorf("안 바뀌었는데 잃었다고 했다: %v", lost)
	}
	// 더하기만 한 것은 막지 않는다
	added := before + "\nmessage NewOne { string x = 1; }\n"
	if lost := lostExportsFor("a.proto", before, added); len(lost) > 0 {
		t.Errorf("더하기만 했는데 막았다: %v", lost)
	}
	// 다른 말에는 계약 규칙을 쓰지 않는다
	if lost := lostExportsFor("x.ts", "export const a = 1;\n", "export const a = 1;\n"); len(lost) > 0 {
		t.Errorf("TS 를 잘못 봤다: %v", lost)
	}
}

// 있는 메시지를 지우지 않은 채 같은 이름으로 하나 더 만드는 일이 잦다.
// 빌드는 계약을 안 보므로 쓰기 전에 본다.
func TestDuplicateProtoDecls(t *testing.T) {
	dup := duplicateDecls("ceoweb/v1/a.proto", `syntax = "proto3";
message ReceivedRequest { string id = 1; }
message Other {}
message ReceivedRequest { string request_id = 1; }
`)
	if len(dup) != 1 || dup[0] != "ReceivedRequest" {
		t.Errorf("중복 선언을 못 잡았다: %v", dup)
	}
	if dup := duplicateDecls("a.proto", "message A {}\nmessage B {}\n"); len(dup) > 0 {
		t.Errorf("겹치지 않는데 막았다: %v", dup)
	}
}
