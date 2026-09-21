package orchestrator

import (
	"strings"
	"testing"
)

// 애매한 답은 아니오다. 「예」 로 읽는 실수가 잘못된 계약을 남긴다.
func TestReadsAsYes(t *testing.T) {
	cases := []struct {
		raw  string
		want bool
	}{
		{"예", true},
		{" 예.\n", true},
		{"yes", true},
		{"Y", true},
		{"아니오", false},
		{"아니다", false},
		{"no", false},
		{"", false},
		{"잘 모르겠다", false},
		{"이 계약은 읽기 전용이므로 예", false}, // 첫 낱말이 예·아니오가 아니다
		{"아니오. 이 계약은 앱과 같은 RPC 다", false},
	}
	for _, c := range cases {
		if got := readsAsYes(c.raw); got != c.want {
			t.Errorf("%q → %v, 기대 %v", c.raw, got, c.want)
		}
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
