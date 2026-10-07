package orchestrator

import (
	"os"
	"strings"
	"testing"
)

// 「본떠라」 라고 말하는 대신 **본뜰 것을 보여 준다.**
//
// 실측 W-14983 이 `ResponseUpdateReceivedRequestHoldStatus` 를 빈 채로 냈다.
// 쪽지에는 "이 계약에 이미 있는 짝을 본떠 필드를 채워라" 고만 적혀 있었다.
func TestEmptyMessageShowsARealPair(t *testing.T) {
	repo := "/home/cnf/cie-repos/proto-ceowebapis"
	if _, err := os.Stat(repo); err != nil {
		t.Skip("계약 사본이 없다")
	}
	diff := "+++ b/ceoweb/v1/x.proto\n+message ResponseUpdateX {\n+}\n"
	bad := emptyNewMessage(repo, diff)
	if len(bad) == 0 {
		t.Fatal("빈 메시지를 안 막았다")
	}
	joined := strings.Join(bad[0].Evidence, "\n")
	if !strings.Contains(joined, "이미 있는 짝") {
		t.Fatalf("본보기를 안 보여 준다:\n%s", joined)
	}
	if !strings.Contains(joined, "message Request") || !strings.Contains(joined, "message Response") {
		t.Fatalf("진짜 짝이 아니다:\n%s", joined)
	}
	t.Logf("\n%s", joined)
}

// 사본이 없어도 전과 같이 동작한다 — 말은 그대로 하고, 본보기만 빠진다.
func TestEmptyMessageWithoutRepoStillWarns(t *testing.T) {
	diff := "+++ b/x.proto\n+message ResponseUpdateX {\n+}\n"
	bad := emptyNewMessage("", diff)
	if len(bad) == 0 {
		t.Fatal("사본이 없다고 안 막으면 안 된다")
	}
	joined := strings.Join(bad[0].Evidence, "\n")
	if strings.Contains(joined, "이미 있는 짝 —") {
		t.Fatal("사본이 없는데 본보기를 지어냈다")
	}
	if !strings.Contains(joined, "부를 수 없다") {
		t.Fatal("까닭을 말하지 않는다")
	}
}
