package agent

import (
	"context"
	"google.golang.org/adk/model"
	"strings"
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
	return a.ProcessWithContext(ctx, diff, "", "")
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
func (a *ReviewerAgent) ProcessWithContext(ctx context.Context, diff, request, analysis string) (string, error) {
	var b strings.Builder
	b.WriteString("You are the Swarm Reviewer.\n")
	b.WriteString("Your goal is to verify the code modifications made by the Coder agent.\n\n")

	if strings.TrimSpace(request) != "" {
		b.WriteString("[무엇을 고쳐 달라고 했나]\n" + clipRunes(request, 600) + "\n\n")
	}
	if strings.TrimSpace(analysis) != "" {
		b.WriteString("[분석이 짚은 원인]\n" + clipRunes(analysis, 1200) + "\n\n")
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
		"   거를 때 그 날이 통째로 빠진다. 경계를 다음 날로 옮기는 것은 옳은 고침이다.\n\n")

	b.WriteString("[Code Changes]\n" + diff)
	return CallLLM(ctx, a.llm, a.Name(), b.String())
}

func (a *ReviewerAgent) IsApproved(resp string) bool {
	upper := strings.ToUpper(resp)
	return strings.Contains(upper, "APPROVED") && !strings.Contains(upper, "FEEDBACK")
}
