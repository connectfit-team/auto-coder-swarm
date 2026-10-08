package orchestrator

import (
	"os"
	"strings"
	"testing"
)

// W-10104 이 관문을 다 지나 승인 대기까지 갔는데 판정 5에서 떨어졌다.
// 판정기가 보는 것을 관문이 안 봤다.
func TestSuffixNameIsCaughtOnThisContract(t *testing.T) {
	repo := "/home/cnf/cie-repos/proto-ceowebapis"
	if _, err := os.Stat(repo); err != nil {
		t.Skip("계약 사본이 없다")
	}
	diff := "+++ b/ceoweb/v1/connect.service.proto\n" +
		"+message UpdateReceivedRequestPendingStatusRequest {\n+  int32 status = 1;\n+}\n" +
		"+message UpdateReceivedRequestPendingStatusResponse {\n+  bool updated = 1;\n+}\n"
	bad := suffixNameOffHabit(repo, diff)
	if len(bad) == 0 {
		t.Fatal("접미형 이름을 안 막았다")
	}
	ev := strings.Join(bad[0].Evidence, "\n")
	// **고친 모양까지 보여 준다** — 이 세션의 교훈이다.
	if !strings.Contains(ev, "RequestUpdateReceivedRequestPendingStatus") {
		t.Fatalf("고친 모양을 안 보여 준다:\n%s", ev)
	}
}

// 접두형으로 바르게 지은 것은 건드리지 않는다.
func TestPrefixNamesPass(t *testing.T) {
	repo := "/home/cnf/cie-repos/proto-ceowebapis"
	if _, err := os.Stat(repo); err != nil {
		t.Skip("계약 사본이 없다")
	}
	diff := "+++ b/ceoweb/v1/connect.service.proto\n" +
		"+message RequestUpdateHoldStatus {\n+  int32 status = 1;\n+}\n" +
		"+message ResponseUpdateHoldStatus {\n+  bool updated = 1;\n+}\n"
	if bad := suffixNameOffHabit(repo, diff); len(bad) > 0 {
		t.Fatalf("바른 이름을 막았다: %s", bad[0].Why)
	}
}

// 접미형을 쓰는 계약에서는 아무 말도 하지 않는다 — 규칙을 박아 두지 않는다.
func TestSuffixContractIsLeftAlone(t *testing.T) {
	repo := protoRepoForTest(t, `syntax = "proto3";
package x.v1;

message GetUserRequest {
  string id = 1;
}
message GetUserResponse {
  string name = 1;
}
`)
	diff := "+++ b/ceoweb/v1/x.proto\n+message MakeThingRequest {\n+  string a = 1;\n+}\n"
	if bad := suffixNameOffHabit(repo, diff); len(bad) > 0 {
		t.Fatalf("접미형 계약에서 접미형을 막았다: %s", bad[0].Why)
	}
}

// 표본이 적으면 말하지 않는다. 한둘을 보고 단정하면 멀쩡한 수정을 문다.
func TestTooFewSamplesStaySilent(t *testing.T) {
	repo := protoRepoForTest(t, `syntax = "proto3";
package x.v1;

message RequestA {
  string a = 1;
}
message ResponseA {
  string b = 1;
}
`)
	diff := "+++ b/ceoweb/v1/x.proto\n+message MakeThingRequest {\n+  string a = 1;\n+}\n"
	if bad := suffixNameOffHabit(repo, diff); len(bad) > 0 {
		t.Fatalf("표본 2개로 단정했다: %s", bad[0].Why)
	}
}

// 사본이 없으면 아무 말도 안 한다.
func TestNoRepoNoOpinion(t *testing.T) {
	if bad := suffixNameOffHabit("", "+message XRequest {\n+}\n"); len(bad) > 0 {
		t.Fatal("사본 없이 판단했다")
	}
}

func TestFlipToPrefix(t *testing.T) {
	for in, want := range map[string]string{
		"UpdateHoldStatusRequest":  "RequestUpdateHoldStatus",
		"UpdateHoldStatusResponse": "ResponseUpdateHoldStatus",
		"RequestAlreadyFine":       "RequestAlreadyFine",
	} {
		if got := flipToPrefix(in); got != want {
			t.Errorf("%q → %q (바라는 것 %q)", in, got, want)
		}
	}
}
