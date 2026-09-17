package orchestrator

import "testing"

// 「기능을 추가할거야」 는 눈이 무엇을 찾았든 새로 만드는 일이다.
func TestNewFeatureRequestDetected(t *testing.T) {
	yes := []string{
		"고용주웹에서, 연결보류 기능을 추가할거야, 관련해서 PR을 올려",
		"근무 유형에 상여금을 추가해줘",
	}
	for _, r := range yes {
		if !IsNewFeatureRequest(r) {
			t.Errorf("새로 만드는 일인데 아니라고 했다: %q", r)
		}
	}
	// 고장 신고는 새로 만드는 일이 아니다.
	no := []string{
		"출퇴근 QR 로 찍었는데 지각으로 기록된다",
		"월별 근무 조회에서 매달 말일 데이터가 통째로 빠진다",
	}
	for _, r := range no {
		if IsNewFeatureRequest(r) {
			t.Errorf("고장 신고를 새로 만드는 일로 봤다: %q", r)
		}
	}
}
