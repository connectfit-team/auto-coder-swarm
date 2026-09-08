package orchestrator

import (
	"strings"
	"testing"
)

func TestIsNewFeatureRequest(t *testing.T) {
	newOnes := []string{
		"고용주웹에서, 연결보류 기능을 추가할거야, 관련해서 PR을 올려",
		"근무 유형에 bonus(상여금) 추가해줘",
		"출퇴근 확인 방식에 BLE 추가해줘",
		"사장님 웹에 급여 내려받기 만들어줘",
		"신규 알림 채널을 구현해줘",
	}
	for _, q := range newOnes {
		if !IsNewFeatureRequest(q) {
			t.Errorf("새로 만드는 일인데 못 알아본다: %s", q)
		}
	}
	// 고치라는 말이 있으면 고장이다 — 있는 코드를 다루는 일이다.
	fixes := []string{
		"cms 의 월별 근무 조회에서 매달 말일 데이터가 통째로 빠진다. 원인을 찾아 고쳐라",
		"출퇴근 QR 로 찍었는데 지각으로 기록된다",
		"푸시 알림이 두 번 온다",
		"버그 고치고 로그도 추가해줘",
	}
	for _, q := range fixes {
		if IsNewFeatureRequest(q) {
			t.Errorf("고장 요청을 새 기능으로 봤다: %s", q)
		}
	}
	if IsNewFeatureRequest("") {
		t.Error("빈 요청을 새 기능으로 봤다")
	}
}

func TestNewFeatureBrief(t *testing.T) {
	b := NewFeatureBrief("연결보류 기능 추가", "연결하는 건 일단 보류하기로 했는데", []string{"internal_v2/business/connect.go"})
	for _, must := range []string{
		"새로 만드는 일이다",
		"본떠",              // 무에서 지어내지 말라는 것이 핵심
		"원문에 있는 코드만 닻으로", // 없는 함수를 찾으라고 하지 말라
		"사내지식",
		"보류하기로 했는데",              // 지식이 실제로 실렸나
		"internal_v2/business/connect.go",
	} {
		if !strings.Contains(b, must) {
			t.Errorf("조건문에 %q 가 없다", must)
		}
	}
	// 지식이 없으면 그 사실을 적어야 한다.
	if !strings.Contains(NewFeatureBrief("x 추가", "", nil), "사내지식을 못 받았다") {
		t.Error("지식 없이 설계하는 것을 안 알린다")
	}
}
