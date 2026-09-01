package orchestrator

import "testing"

func TestLooksLikeDefectRequest(t *testing.T) {
	yes := []string{
		"cms 의 월별 근무 조회에서 매달 말일 데이터가 통째로 빠진다. 원인을 찾아 고쳐라.",
		"8월 31일 기록이 하나도 안 나온다",
		"이 화면에서 오류가 뜬다",
		"정렬이 잘못돼 있다",
	}
	for _, q := range yes {
		if !looksLikeDefectRequest(q) {
			t.Errorf("고장 요청으로 봐야 한다: %q", q)
		}
	}
	no := []string{
		"payslip.ts 에 페이지네이션을 추가해라",
		"이 모듈에 주석을 달아라",
	}
	for _, q := range no {
		if looksLikeDefectRequest(q) {
			t.Errorf("고장 요청이 아니다: %q", q)
		}
	}
}
