package orchestrator

import (
	"strings"
	"testing"
)

func TestMajorityIndexNeedsMoreThanHalf(t *testing.T) {
	cases := []struct {
		name    string
		answers []int
		want    int
	}{
		{"셋 다 같다", []int{2, 2, 2}, 2},
		{"둘이면 과반이다", []int{1, 2, 2}, 2},
		{"갈리면 고르지 않는다", []int{1, 2, 3}, 0},
		{"0 이 많아도 0 은 뽑지 않는다", []int{0, 0, 2}, 0},
		{"0 과 1 이 반반이면 정해지지 않았다", []int{0, 1, 0}, 0},
		{"전부 못 읽었다", []int{0, 0, 0}, 0},
	}
	for _, c := range cases {
		if got, _ := majorityIndex(c.answers); got != c.want {
			t.Errorf("%s: %v → %d, 기대 %d", c.name, c.answers, got, c.want)
		}
	}
}

func TestParseOwnerIndexStaysInRange(t *testing.T) {
	cases := []struct {
		raw  string
		n    int
		want int
	}{
		{"2", 3, 2},
		{" 1.\n", 3, 1},
		{"답: 3 번", 3, 3},
		{"4", 3, 0},  // 목록 밖
		{"0", 3, 0},  // 어느 것도 아니다
		{"없다", 3, 0}, // 번호가 없다
		{"", 3, 0},
	}
	for _, c := range cases {
		if got := parseOwnerIndex(c.raw, c.n); got != c.want {
			t.Errorf("%q → %d, 기대 %d", c.raw, got, c.want)
		}
	}
}

// 물음은 닫혀 있어야 한다 — 후보와 근거를 다 보이고 번호 하나만 받는다.
func TestOwnerPickPromptIsClosed(t *testing.T) {
	order := []string{"proto-ceowebapis", "proto-purchaseapis"}
	ev := map[string]string{
		"proto-ceowebapis":   "a.ts → getConnectClient() → ConnectCEOWebDefinition → src/lib/server/protos/ceowebapis/x.ts",
		"proto-purchaseapis": "b.ts → getPurchaseClient() → PurchaseDefinition → src/lib/server/protos/purchaseapis/y.ts",
	}
	p := ownerPickPrompt(order, ev, []string{"연결 요청에 보류 상태를 담을 필드"})

	for _, must := range []string{
		"1) proto-ceowebapis", "2) proto-purchaseapis",
		"getConnectClient()", "getPurchaseClient()",
		"연결 요청에 보류 상태를 담을 필드",
		"번호 하나만",
	} {
		if !strings.Contains(p, must) {
			t.Errorf("물음에 %q 가 없다", must)
		}
	}
}

// 후보가 여럿이라고 손을 떼면 안 된다 — 거기서 연쇄가 끊겼다.
func TestAmbiguousOwnerIsAsked(t *testing.T) {
	src := readSource(t, "owner_of_state.go")
	if strings.Contains(src, "어느 쪽인지 알 수 없다") {
		t.Error("후보가 여럿일 때 묻지 않고 손을 떼는 옛 모양이 남아 있다")
	}
	if !strings.Contains(src, "pickOwnerAmong") {
		t.Error("후보가 여럿일 때 고르는 물음이 없다")
	}
}

// 걸러낸 저장소는 까닭과 함께 남아야 한다.
func TestRoutingFallbackLogsWhatItDropped(t *testing.T) {
	src := readSource(t, "state_chain.go")
	for _, must := range []string{"dropped", "사본이 없다", "이미 거쳐 온 저장소", "저장소 고르기가 답하지 않았다"} {
		if !strings.Contains(src, must) {
			t.Errorf("state_chain.go 에 %q 가 없다 — 조용히 걸러내면 까닭이 남지 않는다", must)
		}
	}
}
