package orchestrator

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

const fieldOnlyDiff = `+++ b/ceoweb/v1/connect.communication.proto
@@ message ReceivedRequest @@
   string existing_workplace_id = 14;
+
+  int32 hold_status = 15;
 }
`

const fieldAndRPCDiff = fieldOnlyDiff + `+++ b/ceoweb/v1/connect.service.proto
+  rpc UpdateHoldStatus(RequestUpdateHoldStatus) returns (ResponseUpdateHoldStatus) {}
`

// 필드만 넣고 바꾸는 길을 안 낸 것을 잡는다.
//
// 이 세션에서 두 번 그랬고 **그때마다 관문이 아무 말도 안 했다** — 판정기만
// 뒤늦게 잡았다(W-11906 비평가 되돌이, W-23965 「계획에 없는 파일」 되돌리기).
// 쓰는 쪽에서 보면 읽을 수는 있는데 바꿀 수가 없는 계약이다.
func TestFieldWithoutRPCIsCaught(t *testing.T) {
	bad := stateChangeNeedsAnRPC("", fieldOnlyDiff)
	if len(bad) == 0 {
		t.Fatal("필드만 넣었는데 통과시켰다")
	}
	if !strings.Contains(bad[0].Why, "hold_status") {
		t.Fatalf("어느 필드인지 안 밝힌다: %s", bad[0].Why)
	}
}

// 둘 다 넣었으면 아무 말도 안 한다.
func TestFieldWithRPCPasses(t *testing.T) {
	if bad := stateChangeNeedsAnRPC("", fieldAndRPCDiff); len(bad) > 0 {
		t.Fatalf("바른 수정을 막았다: %s", bad[0].Why)
	}
}

// 필드를 넣지도 않았으면 이 검사가 할 말이 없다 — 다른 관문이 말한다.
func TestNoFieldNoOpinion(t *testing.T) {
	if bad := stateChangeNeedsAnRPC("", "+++ b/x.proto\n+// 주석만 바꿨다\n"); len(bad) > 0 {
		t.Fatalf("필드도 없는데 말했다: %s", bad[0].Why)
	}
	if bad := stateChangeNeedsAnRPC("", ""); len(bad) > 0 {
		t.Fatal("빈 diff 에 말했다")
	}
}

// 「더해라」 가 아니라 **어디에** 더하는지 보여 준다.
func TestItShowsWhereToAddTheRPC(t *testing.T) {
	repo := "/home/cnf/cie-repos/proto-ceowebapis"
	if _, err := os.Stat(repo); err != nil {
		t.Skip("계약 사본이 없다")
	}
	bad := stateChangeNeedsAnRPC(repo, fieldOnlyDiff)
	if len(bad) == 0 {
		t.Fatal("필드만 넣었는데 통과시켰다")
	}
	ev := strings.Join(bad[0].Evidence, "\n")
	if !strings.Contains(ev, "service ") || !strings.Contains(ev, ".proto") {
		t.Fatalf("어디에 더하는지 안 보여 준다:\n%s", ev)
	}
	t.Logf("\n%s", ev)
}

// 이번에 고치는 파일의 service 를 가리켜야 한다.
//
// 저장소에서 아무 service 나 집으면 엉뚱한 파일로 보낸다 — 첫 판에서
// `connect.service.proto` 를 고치는 중인데 `ceo.service.proto` 를 가리켰다.
// 자리를 잘못 짚어 주는 것은 이 세션 내내 싸운 실패 모양 그대로다.
func TestHintPointsAtTheFileBeingChanged(t *testing.T) {
	repo := "/home/cnf/cie-repos/proto-ceowebapis"
	if _, err := os.Stat(repo); err != nil {
		t.Skip("계약 사본이 없다")
	}
	diff := fieldOnlyDiff + "+++ b/ceoweb/v1/connect.service.proto\n+  // 여기에 rpc 를 더해야 한다\n"
	bad := stateChangeNeedsAnRPC(repo, diff)
	if len(bad) == 0 {
		t.Fatal("필드만 넣었는데 통과시켰다")
	}
	ev := strings.Join(bad[0].Evidence, "\n")
	if !strings.Contains(ev, "connect.service.proto") {
		t.Fatalf("고치는 파일이 아니라 다른 곳을 가리킨다:\n%s", ev)
	}
	if strings.Contains(ev, "ceo.service.proto") {
		t.Fatalf("엉뚱한 파일을 가리킨다:\n%s", ev)
	}
}

// 「rpc 를 더해라」 만으로는 **어떤 모양인지** 모른다 — 이름을 지어내고 이 계약의
// 버릇과 어긋난다. 실측에서 이 관문이 마지막 남은 실패였다.
func TestNeedRPCShowsTheShape(t *testing.T) {
	repo := gitRepoWith(t, map[string]string{
		"proto/connect.proto": protoPrefixHabit(),
	})
	got := rpcSkeleton(repo, "pending_status")
	for _, want := range []string{
		"RequestUpdatePendingStatus", "ResponseUpdatePendingStatus",
		"rpc UpdatePendingStatus(RequestUpdatePendingStatus) returns (ResponseUpdatePendingStatus);",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("%q 가 없다:\n%s", want, got)
		}
	}
	if rpcSkeleton(repo, "") != "" {
		t.Error("필드가 없는데 모양을 지어낸다")
	}
}

// 접미형을 쓰는 계약이면 접미형으로 지어 준다.
func TestNeedRPCFollowsSuffixHabit(t *testing.T) {
	var b strings.Builder
	b.WriteString("syntax = \"proto3\";\npackage p;\n")
	for i := 1; i <= 13; i++ {
		fmt.Fprintf(&b, "\nmessage Op%dRequest {}\n\nmessage Op%dResponse {}\n", i, i)
	}
	repo := gitRepoWith(t, map[string]string{"proto/a.proto": b.String()})
	if got := rpcSkeleton(repo, "hold_status"); !strings.Contains(got, "UpdateHoldStatusRequest") {
		t.Errorf("접미형 계약인데 접두형으로 지었다:\n%s", got)
	}
}

func protoPrefixHabit() string {
	var b strings.Builder
	b.WriteString("syntax = \"proto3\";\npackage p;\n")
	for i := 1; i <= 13; i++ {
		fmt.Fprintf(&b, "\nmessage RequestOp%d {}\n\nmessage ResponseOp%d {}\n", i, i)
	}
	return b.String()
}
