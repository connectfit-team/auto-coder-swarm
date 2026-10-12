package orchestrator

import (
	"strings"
	"testing"
)

// 섞어 쓴 것을 잡고 **어느 쪽으로 맞추라는 말까지** 한다.
//
// 실측 W-41844 가 `ConnectInviteStatus` 와 `int32` 를 섞어 쓰고 세 시도를 다
// 썼다. 무엇이 틀렸는지는 알아도 어떻게 고치라는 말이 없으면 또 흔들린다.
func TestMixedTypesSayWhichToSettleOn(t *testing.T) {
	diff := "+++ b/ceoweb/v1/x.proto\n" +
		"+  ConnectInviteStatus invite_status = 1;\n" +
		"+  int32 hold_status = 2;\n"
	bad := sameNameDifferentType(diff)
	if len(bad) == 0 {
		t.Fatal("섞어 쓴 것을 안 막았다")
	}
	ev := strings.Join(bad[0].Evidence, "\n")
	if !strings.Contains(ev, "ConnectInviteStatus 쪽으로 모아라") {
		t.Fatalf("어느 쪽으로 맞추라는 말이 없다:\n%s", ev)
	}
}

// 둘 다 글자(스칼라)면 말하지 않는다 — 모을 「만든 타입」 이 없다.
func TestAllScalarsAreLeftAlone(t *testing.T) {
	diff := "+++ b/ceoweb/v1/x.proto\n" +
		"+  int32 hold_status = 1;\n" +
		"+  string other_status = 2;\n"
	if bad := sameNameDifferentType(diff); len(bad) > 0 {
		t.Fatalf("만든 타입이 없는데 말했다: %s", bad[0].Why)
	}
}

// 한 가지 타입만 쓰면 아무 말도 안 한다.
func TestOneTypeIsFine(t *testing.T) {
	diff := "+++ b/ceoweb/v1/x.proto\n" +
		"+  ConnectInviteStatus a_status = 1;\n" +
		"+  ConnectInviteStatus b_status = 2;\n"
	if bad := sameNameDifferentType(diff); len(bad) > 0 {
		t.Fatalf("한 가지로 맞췄는데 막았다: %s", bad[0].Why)
	}
}
