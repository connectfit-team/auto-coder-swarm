package orchestrator

import "testing"

// 틀린 답이 **서로 다르다**는 것이 이 수정의 전제다.
//
// 실측: 뿌리 작업 22번 가운데 20번은 `ReceivedRequest`, 한 번은 `TradeInfo`,
// 한 번은 `InviteCandidate` 였다. 같은 오답으로 몰리지 않으므로 다수결이 듣는다.
func TestMajorityBeatsOneBadDraw(t *testing.T) {
	got, votes := majorityOf([]string{"ReceivedRequest", "TradeInfo", "ReceivedRequest"})
	if got != "ReceivedRequest" || votes != 2 {
		t.Fatalf("%q %d표", got, votes)
	}
}

// 셋이 다 다르면 과반이 없다 — 먼저 나온 것을 고르고, 부르는 쪽이
// STATE_TYPE_SPLIT 으로 남긴다.
func TestAllDifferentPicksTheFirst(t *testing.T) {
	got, votes := majorityOf([]string{"A", "B", "C"})
	if got != "A" || votes != 1 {
		t.Fatalf("%q %d표", got, votes)
	}
}

// 동점이면 먼저 나온 것. map 순회로 고르면 회차마다 답이 달라져 두 판을
// 견줄 수 없다.
func TestTieIsStable(t *testing.T) {
	for i := 0; i < 50; i++ {
		if got, _ := majorityOf([]string{"B", "A", "B", "A"}); got != "B" {
			t.Fatalf("동점인데 답이 흔들린다: %q", got)
		}
	}
}

func TestNoAnswersGivesNothing(t *testing.T) {
	if got, votes := majorityOf(nil); got != "" || votes != 0 {
		t.Fatalf("%q %d", got, votes)
	}
}

func TestOneAnswerIsTheAnswer(t *testing.T) {
	if got, votes := majorityOf([]string{"ReceivedRequest"}); got != "ReceivedRequest" || votes != 1 {
		t.Fatalf("%q %d", got, votes)
	}
}
