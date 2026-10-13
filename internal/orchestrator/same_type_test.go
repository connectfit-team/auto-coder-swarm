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

// 꾸러미를 붙인 이름은 **같은 타입**이다.
//
// `PendingStatus` 와 `ceo.connect.ceoweb.PendingStatus` 는 proto 에서 동일하다.
// 글자로 견주면 「서로 다르게 다룬다」 가 되어 **멀쩡한 수정을 막는다** —
// 실측 W-13441 이 그렇게 세 시도를 다 썼고, 쪽지는
// 「PendingStatus · ceo.connect.ceoweb.PendingStatus 쪽으로 모아라」 라는
// 말이 안 되는 말을 했다.
func TestQualifiedNameIsTheSameType(t *testing.T) {
	diff := "+++ b/ceoweb/v1/x.proto\n" +
		"+  PendingStatus pending_status = 15;\n" +
		"+  ceo.connect.ceoweb.PendingStatus new_status = 2;\n"
	if bad := sameNameDifferentType(diff); len(bad) > 0 {
		t.Fatalf("같은 타입을 다르다고 했다: %s", bad[0].Why)
	}
}

// 진짜로 다른 타입은 그대로 문다 — 이 관문이 있는 까닭이다.
func TestGenuinelyDifferentTypesStillCaught(t *testing.T) {
	diff := "+++ b/ceoweb/v1/x.proto\n" +
		"+  ceoweb.v1.ConnectInviteStatus invite_status = 1;\n" +
		"+  int32 hold_status = 2;\n"
	bad := sameNameDifferentType(diff)
	if len(bad) == 0 {
		t.Fatal("정말 다른 타입을 안 막았다")
	}
	// 쪽지에도 꾸러미 없는 이름으로 적는다.
	ev := strings.Join(bad[0].Evidence, "\n")
	if !strings.Contains(ev, "ConnectInviteStatus 쪽으로 모아라") {
		t.Fatalf("어느 쪽으로 모으라는지 안 밝힌다:\n%s", ev)
	}
}

// 「모아라」 만으로는 **어느 필드인지** 모른다. 고칠 자리를 적어 준다.
func TestSameTypeNamesTheFieldsToFix(t *testing.T) {
	diff := "--- a/x.proto\n+++ b/x.proto\n@@\n" +
		"+  ConnectPendingStatus pending_status = 3;\n" +
		"+  int32 invite_status = 4;\n" +
		"+  string hold_status = 5;\n"

	bad := sameNameDifferentType(diff)
	if len(bad) == 0 {
		t.Fatal("섞어 쓴 것을 못 잡았다")
	}
	note := AlignmentNote(bad)
	if !strings.Contains(note, "고칠 자리:") {
		t.Fatalf("고칠 자리를 안 적는다:\n%s", note)
	}
	for _, want := range []string{"int32 invite_status", "string hold_status"} {
		if !strings.Contains(note, want) {
			t.Errorf("%q 를 안 짚는다:\n%s", want, note)
		}
	}
	// 남길 쪽은 고칠 자리에 없어야 한다.
	if strings.Contains(note, "고칠 자리:") &&
		strings.Contains(note[strings.Index(note, "고칠 자리:"):], "ConnectPendingStatus pending_status") {
		t.Errorf("남길 쪽까지 고치라고 한다:\n%s", note)
	}
}

// 「필드 note = 2」 만으로는 **무엇으로 되살릴지** 모른다 — 타입이 빠져 있다.
func TestChangedExistingGivesTheOriginalLine(t *testing.T) {
	diff := "--- a/x.proto\n+++ b/x.proto\n@@\n" +
		"-  string note = 2;\n" +
		"+  bool note = 2;\n" +
		"+  bool held = 3;\n"

	bad := changedExisting(diff)
	if len(bad) == 0 {
		t.Fatal("있던 것을 고친 것을 못 잡았다")
	}
	note := AlignmentNote(bad)
	for _, want := range []string{"원래 줄: `string note = 2;`", "원래 줄을 그대로 되살려라", "새 번호로"} {
		if !strings.Contains(note, want) {
			t.Errorf("%q 가 없다:\n%s", want, note)
		}
	}
}

// 「있는 service 안에 더해라」 만으로는 **무엇을 옮길지** 모른다.
func TestNewServiceSaysWhichRPCsToMove(t *testing.T) {
	repo := gitRepoWith(t, map[string]string{
		"proto/service.proto": "syntax = \"proto3\";\npackage p;\n\n" +
			"message RequestGet {}\n\nmessage ResponseGet {}\n\n" +
			"service ConnectService {\n  rpc Get(RequestGet) returns (ResponseGet);\n}\n",
	})
	diff := "--- a/proto/service.proto\n+++ b/proto/service.proto\n@@\n" +
		"+service HoldService {\n" +
		"+  rpc HoldConnect(RequestHold) returns (ResponseHold);\n" +
		"+}\n"

	bad := newServiceBesideOne(repo, []string{"proto/service.proto"}, diff)
	if len(bad) == 0 {
		t.Fatal("새 service 를 못 잡았다")
	}
	note := AlignmentNote(bad)
	for _, want := range []string{"service ConnectService 안으로 옮겨라", "rpc HoldConnect", "통째로 지워라"} {
		if !strings.Contains(note, want) {
			t.Errorf("%q 가 없다:\n%s", want, note)
		}
	}
}
