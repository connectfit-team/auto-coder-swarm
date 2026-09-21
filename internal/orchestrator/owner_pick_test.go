package orchestrator

import (
	"strings"
	"testing"
)

// 「어느 것도 아니다」 와 「못 정했다」 는 다른 답이다. 섞으면 모델이 경고를
// 읽고 거절했는데도 따라간 계약이 확정된다.
func TestMajorityIndexTellsRefusalFromIndecision(t *testing.T) {
	cases := []struct {
		name    string
		answers []int
		pick    int
		decided bool
	}{
		{"셋 다 같다", []int{2, 2, 2}, 2, true},
		{"둘이면 과반이다", []int{1, 2, 2}, 2, true},
		{"갈리면 정해지지 않았다", []int{1, 2, 3}, 0, false},
		{"0 이 과반이면 거절이다", []int{0, 0, 2}, 0, true},
		{"0 이 과반이면 거절이다(0,1,0)", []int{0, 1, 0}, 0, true},
		{"전부 거절", []int{0, 0, 0}, 0, true},
		{"둘씩 갈리면 정해지지 않았다", []int{1, 2, 3, 4}, 0, false},
	}
	for _, c := range cases {
		pick, _, decided := majorityIndex(c.answers)
		if pick != c.pick || decided != c.decided {
			t.Errorf("%s: %v → (%d, %v), 기대 (%d, %v)", c.name, c.answers, pick, decided, c.pick, c.decided)
		}
	}
}

// 번호가 아닌 답에서 숫자를 주워 오면 조용히 엉뚱한 계약이 뽑힌다.
func TestParseOwnerPickRejectsStrayNumbers(t *testing.T) {
	order := []string{
		"src/lib/server/protos/attendanceapis/attendance/v2/ceoweb.internal.ts",
		"src/lib/server/protos/ceowebapis/ceoweb/v1/ceo.service.ts",
		"src/lib/server/protos/ceowebapis/ceoweb/v1/connect.service.ts",
	}
	cases := []struct {
		raw  string
		want int
	}{
		{"2", 2},
		{" 3.\n", 3},
		{"1번", 1},
		{"답: 3", 3},
		{"3)", 3},
		{"4", 0},  // 목록 밖
		{"0", 0},  // 어느 것도 아니다
		{"없다", 0}, // 번호가 없다
		{"", 0},
		// 아래가 첫 정수 줍기로는 전부 엉뚱한 번호가 되던 답이다.
		{"17개 후보 가운데 3번", 0},
		{"1번은 아니고 3번이다", 0},
		{"v1 의 connect.service.ts", 0},
		{"src/lib/server/protos/ceowebapis/ceoweb/v1/connect.service.ts", 3},
		{"연결 요청이므로 src/lib/server/protos/ceowebapis/ceoweb/v1/connect.service.ts 입니다", 3},
	}
	for _, c := range cases {
		if got := parseOwnerPick(c.raw, order); got != c.want {
			t.Errorf("%q → %d, 기대 %d", c.raw, got, c.want)
		}
	}
}

// 물음은 닫혀 있어야 하고, 그 계약에 적힌 말이 실려야 한다.
func TestContractPickPromptIsClosed(t *testing.T) {
	order := []string{
		"src/lib/server/protos/ceowebapis/ceoweb/v1/ceo.service.ts",
		"src/lib/server/protos/ceowebapis/ceoweb/v1/connect.service.ts",
	}
	ev := map[string]string{
		order[0]: "staff.ts → getStaffClient() → StaffInternalDefinition (proto-ceowebapis)\n     그 계약에 적힌 말: 읽기 전용이다. 조회에는 쓰지 마라",
		order[1]: "connect.ts → getConnectClient() → ConnectCEOWebDefinition (proto-ceowebapis)",
	}
	p := contractPickPrompt(order, ev, []string{"연결 요청에 보류 상태를 담을 필드"})
	for _, must := range []string{
		"1) " + order[0], "2) " + order[1],
		"getStaffClient()", "getConnectClient()",
		"읽기 전용이다. 조회에는 쓰지 마라",
		"연결 요청에 보류 상태를 담을 필드",
		"번호 하나만",
	} {
		if !strings.Contains(p, must) {
			t.Errorf("물음에 %q 가 없다", must)
		}
	}
}

// 후보가 여럿이라고 손을 떼면 연쇄가 끊긴다. 하나뿐이어도 묻는다.
func TestAmbiguousOwnerIsAsked(t *testing.T) {
	src := readSource(t, "owner_of_state.go")
	if strings.Contains(src, "어느 쪽인지 알 수 없다") {
		t.Error("후보가 여럿일 때 묻지 않고 손을 떼는 옛 모양이 남아 있다")
	}
	for _, must := range []string{"pickContractAmong", "traceContracts"} {
		if !strings.Contains(src, must) {
			t.Errorf("owner_of_state.go 에 %q 가 없다", must)
		}
	}
	if !strings.Contains(src, "scan.complete()") {
		t.Error("잘린 목록을 온전한 것처럼 내놓는다")
	}
	// 고르지 못했다고 손을 떼면 연쇄가 끊긴다 — 옛 길로 내려가야 한다.
	if !strings.Contains(src, "CONTRACT_PICK_NONE") {
		t.Error("못 골랐을 때 타입을 묻는 길로 내려가지 않는다")
	}
	// 「어느 것도 아니다」 라고 답했으면 따라간 것을 대신 쓰면 안 된다.
	if !strings.Contains(src, "!rejected && len(traced) == 1") {
		t.Error("거절을 미결정과 같이 다뤄 따라간 계약이 확정된다")
	}
	// 따라간 것에도 계약에 적힌 말을 옮겨야 한다.
	if !strings.Contains(src, "c.note = from.note") {
		t.Error("따라간 계약에 경고가 실리지 않는다")
	}
}

func TestRoutingFallbackLogsWhatItDropped(t *testing.T) {
	src := readSource(t, "state_chain.go")
	for _, must := range []string{"dropped", "사본이 없다", "이미 거쳐 온 저장소", "저장소 고르기가 답하지 않았다"} {
		if !strings.Contains(src, must) {
			t.Errorf("state_chain.go 에 %q 가 없다 — 조용히 걸러내면 까닭이 남지 않는다", must)
		}
	}
}
