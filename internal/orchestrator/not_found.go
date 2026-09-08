package orchestrator

import (
	"regexp"
	"strings"
)

// 눈이 "못 찾았다" 고 하면 손이 멈춰야 한다.
//
// "고용주웹에서 연결보류 기능을 추가할거야" 에 CIE 는 정직하게 답했다.
//
//	확인하지 못한 내용:
//	- "연결 보류", "pending connection", "hold connection" 키워드로는
//	  관련 코드를 찾지 못했습니다.
//	- 구체적인 "연결 보류" 기능 자체에 대한 구현 코드나 파일은 확인하지
//	  못했습니다.
//
// 그런데 전략 단계는 `가능=true` 를 냈고, 계획을 세우고 코드를 썼다. 있지도
// 않은 함수를 찾으라고 한 것(W-70980)의 뿌리가 여기다 — **눈이 없다고 한
// 것을 손이 지어냈다.**
//
// 없는 기능을 새로 만드는 것 자체는 할 수 있는 일이다. 그러나 그것은
// 「고쳐라」 가 아니라 「설계해라」 이고, 설계는 사람이 정한다. 조용히
// 지어내는 것이 가장 나쁘다.

var notFoundPhrases = []*regexp.Regexp{
	regexp.MustCompile(`찾지\s*못했`),
	regexp.MustCompile(`확인하지\s*못했`),
	regexp.MustCompile(`관련\s*코드가?\s*없`),
	regexp.MustCompile(`구현(된)?\s*(코드|파일)[가는]?\s*(확인|발견)?되?지?\s*않`),
	regexp.MustCompile(`존재하지\s*않`),
	regexp.MustCompile(`(?i)could\s+not\s+find`),
	regexp.MustCompile(`(?i)no\s+(relevant\s+)?(code|implementation)\s+found`),
}

// 반대 신호. 「찾았다」 가 함께 있으면 부분적으로는 찾은 것이다.
var foundPhrases = []*regexp.Regexp{
	regexp.MustCompile(`짚은 것들의 정의 위치`),
	regexp.MustCompile(`찾았습니다|발견했습니다|확인했습니다`),
}

// AnalysisSaysNotFound 는 분석이 "그 기능을 못 찾았다" 고 말했는지 본다.
//
// 판단이 아니라 글자다 — 모델에게 "이 분석이 쓸모 있나" 를 되묻지 않는다.
// 되물으면 그 답이 또 틀릴 수 있고, 틀린 답을 검사할 방법이 없다.
func AnalysisSaysNotFound(analysis string) (string, bool) {
	if strings.TrimSpace(analysis) == "" {
		return "", false
	}
	var hits []string
	for _, line := range strings.Split(analysis, "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		for _, re := range notFoundPhrases {
			if re.MatchString(t) {
				hits = append(hits, strings.TrimLeft(t, "-*# "))
				break
			}
		}
	}
	if len(hits) == 0 {
		return "", false
	}
	// 한 줄만 걸리고 「찾았다」 가 함께 있으면 부분적으로 찾은 것이다.
	// 그런 것으로 작업을 세우면 멀쩡한 일도 못 하게 된다.
	if len(hits) < 2 {
		for _, re := range foundPhrases {
			if re.MatchString(analysis) {
				return "", false
			}
		}
	}
	if len(hits) > 3 {
		hits = hits[:3]
	}
	return strings.Join(hits, "\n"), true
}
