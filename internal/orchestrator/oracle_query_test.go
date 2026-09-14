package orchestrator

import (
	"strings"
	"testing"
)

// 분석 질의는 사람의 말과 저장소 이름을 잃으면 안 된다.
//
// 다듬은 질문이 표 질문으로 읽혀, gig_ceo_web 을 물었는데
// cms/prisma/attendance.schema.prisma 의 표 정의가 답으로 왔다(W-48189).
func TestOracleQueryKeepsRequestAndRepo(t *testing.T) {
	cases := []struct{ repo, req string }{
		{"gig_ceo_web", "고용주웹에서, 연결보류 기능을 추가할거야, 관련해서 PR을 올려"},
		{"cms", "cms 의 월별 근무 조회에서 매달 말일 데이터가 통째로 빠진다"},
		{"attendance-api", "출퇴근 QR 로 찍었는데 지각으로 기록된다"},
	}
	for _, c := range cases {
		q := oracleQueryFor(c.repo, c.req)
		if !strings.Contains(q, c.repo) {
			t.Errorf("%q: 저장소 이름이 없다 — %q", c.req, q)
		}
		if !strings.Contains(q, strings.TrimSpace(c.req)) {
			t.Errorf("%q: 사람의 말이 없다 — %q", c.req, q)
		}
	}
}
