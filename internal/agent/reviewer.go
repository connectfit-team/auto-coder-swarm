package agent

import (
	"context"
	"strings"

	"google.golang.org/adk/model"
)

type ReviewerAgent struct {
	llm model.LLM
}

func NewReviewerAgent(m model.LLM) *ReviewerAgent {
	return &ReviewerAgent{llm: m}
}

func (a *ReviewerAgent) Name() string {
	return "Reviewer"
}

func (a *ReviewerAgent) Process(ctx context.Context, diff string) (string, error) {
	return a.ProcessWithContext(ctx, diff, "", "", "")
}

// ProcessWithContext 는 **왜 그렇게 고쳤는지**를 함께 보고 검토한다.
//
// 근거 없이 diff 만 보면 맞는 수정을 되돌리라고 한다. 실측으로 말일 경계
// 버그를 정확히 고친 최소 수정(파일 1개)을 반려하고, 그다음 시도에서 나온
// 파일 6개짜리(넷은 요청과 무관)를 통과시켰다. 반려 이유는 이랬다 —
// "new Date(y, m+1, 0) 은 이번 달 말일을 얻는 올바른 방법이니 되돌려라".
// 맞는 말이지만 lt 가 그 말일 00:00 을 경계로 삼는다는 것을 놓쳤다.
//
// 요청문과 분석이 짚은 원인을 같이 주면 그 판단을 할 수 있다.
func (a *ReviewerAgent) ProcessWithContext(ctx context.Context, diff, request, analysis, facts string) (string, error) {
	var b strings.Builder
	b.WriteString("You are the Swarm Reviewer.\n")
	b.WriteString("Your goal is to verify the code modifications made by the Coder agent.\n\n")

	if strings.TrimSpace(request) != "" {
		b.WriteString("[무엇을 고쳐 달라고 했나]\n" + clipRunes(request, 600) + "\n\n")
	}
	if strings.TrimSpace(analysis) != "" {
		b.WriteString("[분석이 짚은 원인]\n" + clipRunes(analysis, 1200) + "\n\n")
	}
	// **의견 말고 사실을 준다.**
	//
	// 검토자는 이름이 있는지, 시킨 일을 했는지 알 방법이 없어서 늘 인상으로
	// 답했다 — 라벨 붙인 문항 셋에서 0/3 이었고, 나쁜 것은 통과시키고 좋은
	// 것은 막았다. 기계가 파일을 읽어 센 것을 함께 주면 그 둘은 **확인**이
	// 된다.
	if strings.TrimSpace(facts) != "" {
		b.WriteString("[기계가 파일을 읽어 확인한 사실 — 여기 적힌 것은 추측이 아니다]\n" +
			clipRunes(facts, 2000) + "\n\n")
	}

	b.WriteString("MANDATORY RULES:\n" +
		"1. Output 'APPROVED' if the changes are correct and follow conventions.\n" +
		"2. Only if you found a concrete problem **in this diff**, start with 'FEEDBACK:'\n" +
		"   and name the file and line. If you cannot point to a file and line that\n" +
		"   appears in the diff above, output APPROVED instead.\n" +
		"3. Do not discuss code that is not in the diff.\n" +
		"4. A reviewer who always finds something is worse than no reviewer.\n" +
		"5. 위에 적힌 요청과 원인에 **맞는** 수정이면 승인해라. 다른 방식이\n" +
		"   더 낫다는 이유로 반려하지 마라 — 되돌리면 원래 버그가 그대로 남는다.\n" +
		"6. 경계값을 볼 때 조심해라. 비교 대상이 그 날 00:00 이면 \"미만\"으로\n" +
		"   거를 때 그 날이 통째로 빠진다. 경계를 다음 날로 옮기는 것은 옳은 고침이다.\n" +
		"7. **없는 이름을 부르면 반려해라.** 위 [사실] 의 '있는 이름' 목록에\n" +
		"   없는 메서드·내보내기를 diff 가 부르면, 그 줄을 대고 FEEDBACK 해라.\n" +
		"8. **시킨 일을 안 했으면 반려해라.** 계획이 짚은 자리를 하나도 안\n" +
		"   고치고 상관없는 정리만 있으면 FEEDBACK 해라.\n" +
		"9. 시험 파일을 더한 것은 흠이 아니다. 그것만으로 반려하지 마라.\n\n")

	b.WriteString("[Code Changes]\n" + diff)
	return CallLLM(ctx, a.llm, a.Name(), b.String())
}

func (a *ReviewerAgent) IsApproved(resp string) bool {
	upper := strings.ToUpper(resp)
	return strings.Contains(upper, "APPROVED") && !strings.Contains(upper, "FEEDBACK")
}
