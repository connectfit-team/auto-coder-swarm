package orchestrator

import (
	"strings"
	"testing"
)

// 애매한 답은 아니오다. 「예」 로 읽는 실수가 잘못된 계약을 남긴다.
func TestReadsAsYes(t *testing.T) {
	cases := []struct {
		raw  string
		yes  bool
		read bool
	}{
		{"예", true, true},
		{" 예.\n", true, true},
		{"yes", true, true},
		{"Y", true, true},
		{"아니오", false, true},
		{"아니다", false, true},
		{"no", false, true},
		{"", false, false},
		{"잘 모르겠다", false, false},
		{"이 계약은 읽기 전용이므로 예", false, false},
		{"아니오. 이 계약은 앱과 같은 RPC 다", false, true},
	}
	for _, c := range cases {
		yes, read := readsAsYes(c.raw)
		if yes != c.yes || read != c.read {
			t.Errorf("%q → (%v, %v), 기대 (%v, %v)", c.raw, yes, read, c.yes, c.read)
		}
	}
}

// 떨어뜨리려면 「아니오」 가 과반이어야 한다. 못 읽은 답을 아니오로 세면
// 말이 많은 회차마다 후보가 통째로 사라진다 — 실측으로 17개 중 0개가
// 남은 회차가 있었다.
func TestUnreadableAnswerKeepsTheCandidate(t *testing.T) {
	src := readSource(t, "shortlist.go")
	if !strings.Contains(src, "no*2 <= rounds") {
		t.Error("못 읽은 답을 아니오로 세고 있다")
	}
	if strings.Contains(src, "return yes*2 > rounds") {
		t.Error("「예」 과반을 요구하는 옛 모양이 남아 있다")
	}
}

// 물음에는 그 계약의 근거와 적힌 말이 실려야 하고, 쓰지 말라는 표시를
// 아니오로 보라고 일러 줘야 한다.
func TestContractFitPromptCarriesTheWarning(t *testing.T) {
	p := contractFitPrompt(
		"src/lib/server/protos/workstampapis/workstamp/v1/service.ts",
		"getWorkStampAppClient() → WorkStampServiceDefinition (proto-workstampapis)\n     그 계약에 적힌 말: 🚨 근무 조회·쓰기에는 쓰지 마라.",
		[]string{"연결 요청에 보류 상태를 담을 필드"})
	for _, must := range []string{
		"workstampapis/workstamp/v1/service.ts",
		"근무 조회·쓰기에는 쓰지 마라",
		"연결 요청에 보류 상태를 담을 필드",
		"예 또는 아니오",
		"쓰지 마라",
	} {
		if !strings.Contains(p, must) {
			t.Errorf("물음에 %q 가 없다", must)
		}
	}
}

// 후보가 많으면 하나씩 묻는 길로 가야 한다.
func TestBigMenuIsSplit(t *testing.T) {
	src := readSource(t, "owner_pick.go")
	if !strings.Contains(src, "shortlistContracts") {
		t.Error("후보가 많아도 한 번에 고르라고 묻는다")
	}
	if !strings.Contains(src, "len(order) > shortlistFrom") {
		t.Error("쪼개는 기준이 없다")
	}
}
