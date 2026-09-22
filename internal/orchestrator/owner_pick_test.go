package orchestrator

import (
	"strings"
	"testing"
)

// 점수 답은 첫 줄의 숫자 하나일 때만 읽는다.
func TestParseScore(t *testing.T) {
	cases := []struct {
		raw  string
		n    int
		read bool
	}{
		{"3", 3, true},
		{" 0 \n", 0, true},
		{"10", 10, true},
		{"11", 0, false}, // 범위 밖
		{"", 0, false},
		{"잘 모르겠다", 0, false},
		{"src/lib/server/protos/ceowebapis/ceoweb/v1/connect.service.ts", 0, false},
		{"3점 정도가 적당하다고 봅니다", 0, false}, // 숫자가 하나여도 문장은 안 읽는다
		{"v1 의 3", 0, false},
	}
	for _, c := range cases {
		n, read := parseScore(c.raw)
		if n != c.n || read != c.read {
			t.Errorf("%q → (%d, %v), 기대 (%d, %v)", c.raw, n, read, c.n, c.read)
		}
	}
}

// 가장 높은 하나를 고르되, 같은 점수가 둘이면 고르지 않는다.
func TestBestByScore(t *testing.T) {
	order := []string{"a", "b", "c"}
	cases := []struct {
		name   string
		scores map[string]int
		best   string
		top    int
		read   bool
	}{
		{"하나가 높다", map[string]int{"a": 0, "b": 3, "c": 0}, "b", 3, true},
		{"같은 점수가 둘", map[string]int{"a": 3, "b": 3, "c": 0}, "", 3, true},
		{"모두 0", map[string]int{"a": 0, "b": 0, "c": 0}, "", 0, true},
		{"하나도 못 읽음", map[string]int{"a": -1, "b": -1, "c": -1}, "", -1, false},
		{"못 읽은 것은 건너뛴다", map[string]int{"a": -1, "b": 2, "c": -1}, "b", 2, true},
	}
	for _, c := range cases {
		best, top, _, read := bestByScore(order, c.scores)
		if best != c.best || top != c.top || read != c.read {
			t.Errorf("%s: (%q,%d,%v), 기대 (%q,%d,%v)", c.name, best, top, read, c.best, c.top, c.read)
		}
	}
}

// 물음에는 그 계약의 근거와 적힌 말이 실려야 한다.
func TestContractScorePromptCarriesTheWarning(t *testing.T) {
	p := contractScorePrompt(
		"src/lib/server/protos/workstampapis/workstamp/v1/service.ts",
		"getWorkStampAppClient() → WorkStampServiceDefinition (proto-workstampapis)\n     그 계약에 적힌 말: 🚨 근무 조회·쓰기에는 쓰지 마라.",
		[]string{"연결 요청에 보류 상태를 담을 필드"})
	for _, must := range []string{
		"workstampapis/workstamp/v1/service.ts",
		"근무 조회·쓰기에는 쓰지 마라",
		"연결 요청에 보류 상태를 담을 필드",
		"0 부터 10",
	} {
		if !strings.Contains(p, must) {
			t.Errorf("물음에 %q 가 없다", must)
		}
	}
}

// 예·아니오로 묻지 않는다 — 같은 모델이 무엇에든 아니오라고 답했다.
func TestPickDoesNotAskYesNo(t *testing.T) {
	for _, f := range []string{"owner_pick.go", "shortlist.go"} {
		src := readSource(t, f)
		if strings.Contains(src, "예 또는 아니오") {
			t.Errorf("%s 가 아직 예·아니오로 묻는다", f)
		}
	}
	if !strings.Contains(readSource(t, "owner_pick.go"), "scoreContracts") {
		t.Error("점수로 고르지 않는다")
	}
	// 물음에 단서를 덧붙이면 정답까지 0점이 된다 — 재어서 확인했다.
	// 소스가 아니라 **만들어진 물음**을 본다. 주석은 그 사실을 적어 두는
	// 자리이므로 거기 같은 낱말이 있어도 물음이 아니다.
	p := contractScorePrompt("x.ts", "근거", []string{"무엇"})
	if strings.Contains(p, "쓰지 말라고") {
		t.Error("점수 물음에 단서가 들어갔다 — 그 줄 하나로 후보가 모두 0점이 된다")
	}
}

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
	if !strings.Contains(src, "CONTRACT_PICK_NONE") {
		t.Error("못 골랐을 때 타입을 묻는 길로 내려가지 않는다")
	}
	if !strings.Contains(src, "!rejected && len(traced) == 1") {
		t.Error("거절을 미결정과 같이 다뤄 따라간 계약이 확정된다")
	}
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
