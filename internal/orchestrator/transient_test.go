package orchestrator

import (
	"errors"
	"testing"
)

func TestIsTransient(t *testing.T) {
	yes := []string{
		"분석 실패: 서버 재시작으로 중단됨",
		"dial tcp 127.0.0.1:8005: connect: connection refused",
		"Post \"http://x/analyze\": context deadline exceeded",
		"oracle returned error status: 503",
	}
	for _, s := range yes {
		if !isTransient(errors.New(s)) {
			t.Errorf("다시 물어보면 될 것인데 아니라고 한다: %s", s)
		}
	}
	// 코드가 틀린 것은 다시 물어도 같다 — 되풀이하면 시간만 쓴다.
	no := []string{
		"이 저장소에서 그 기능을 찾지 못했다",
		"계획한 파일 3개 가운데 1개를 고치지 못했다",
		"원문에 없는 내용을 찾으라고 했다",
	}
	for _, s := range no {
		if isTransient(errors.New(s)) {
			t.Errorf("코드 문제인데 다시 물어보려 한다: %s", s)
		}
	}
	if isTransient(nil) {
		t.Error("오류가 없는데 다시 물어보려 한다")
	}
}
