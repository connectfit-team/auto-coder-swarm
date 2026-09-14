package orchestrator

import "strings"

// 배포로 끊긴 것은 코드 문제가 아니다.
//
// CIE 를 배포하면 돌던 분석이 끊기고, 그 작업은 이렇게 끝난다.
//
//	분석 실패: 서버 재시작으로 중단됨
//
// 이 기계는 하루 세 번 다시 뜨고, 배포도 수시로 한다. 몇 분짜리 분석이
// 거기에 걸릴 확률은 낮지 않다 — 실측으로 W-36819 가 그렇게 죽었다.
// 사람이 요청을 다시 넣어야 하는데, 다시 넣으면 그냥 된다.
//
// 다시 물어보면 되는 것과 물어봐야 소용없는 것을 가른다. 한 번만 다시
// 물어본다 — 정말로 안 되는 것이면 두 번째도 같은 답이 온다.

var transientWords = []string{
	"서버 재시작", "restart", "connection refused", "connection reset",
	"context deadline exceeded", "EOF", "timeout", "temporarily unavailable",
	"503", "502", "504",
}

// isTransient 는 다시 물어보면 될 실패인지 본다.
func isTransient(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	for _, w := range transientWords {
		if strings.Contains(s, strings.ToLower(w)) {
			return true
		}
	}
	return false
}
