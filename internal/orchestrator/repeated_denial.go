package orchestrator

import (
	"fmt"
	"strings"
)

// 같은 자리에서 같은 이유로 되풀이 막히는 것을 센다.
//
// 실측 W-33314·W-24124 는 세 시도 내내 같은 말을 들었다 —
// "service ConnectService 를 새로 만들었다, 이 파일에는 이미 service Internal
// 가 있다". 그런데 사람에게 간 말은 **"최대 시도 초과"** 한 줄이었다. 무엇이
// 막았는지 모르면 고칠 데도 모른다.
//
// 끊지는 않는다. 세 번은 이미 상한이고, 도중에 끊으면 마지막 시도에서 되는
// 경우(실측 W-99311)를 잃는다. 대신 **무엇이 되풀이됐는지 남긴다.**
// hermes 의 denial_circuit_breaker 가 세는 것과 같은 것을 센다.

// noteFeedback 은 이번 시도를 되돌린 까닭을 센다.
func (t *taskContext) noteFeedback() {
	reason := denialReason(t.lastFeedback)
	if reason == "" {
		return
	}
	if t.denials == nil {
		t.denials = map[string]int{}
	}
	t.denials[reason]++
	if t.denials[reason] == 2 {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "DENIAL_REPEATED",
			"같은 이유로 두 번 막혔다 — 되먹임이 닿지 않고 있다", "", reason)
	}
}

// repeatedDenial 은 가장 많이 되풀이된 까닭과 횟수를 준다.
func (t *taskContext) repeatedDenial() (string, int) {
	best, n := "", 0
	for reason, count := range t.denials {
		// 횟수가 같으면 글자 순으로 골라 매번 같은 답이 나오게 한다.
		if count > n || (count == n && reason < best) {
			best, n = reason, count
		}
	}
	return best, n
}

// denialReason 은 되먹임에서 **되풀이를 셀 수 있는 부분**만 뽑는다.
//
// 되먹임에는 시도마다 달라지는 꼬리가 붙는다(남은 오류 개수, 되살린 시도
// 번호). 그것까지 넣고 세면 같은 이유도 늘 새 것으로 보인다.
func denialReason(feedback string) string {
	head := strings.TrimSpace(feedback)
	if head == "" {
		return ""
	}
	if i := strings.Index(head, "\n"); i > 0 {
		head = head[:i]
	}
	head = strings.TrimSpace(head)
	if len([]rune(head)) > 160 {
		head = string([]rune(head)[:160])
	}
	return head
}

// whyStuck 은 되풀이된 까닭을 사람이 읽을 한 줄로 만든다.
func (t *taskContext) whyStuck() error {
	reason, n := t.repeatedDenial()
	if n < 2 {
		return nil
	}
	return fmt.Errorf("같은 이유로 %d번 막혔다 — %s", n, reason)
}
