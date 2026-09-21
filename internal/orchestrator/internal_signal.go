package orchestrator

import (
	"errors"
	"fmt"
	"strings"
)

// 안쪽 신호는 흐름 안에서만 뜻이 있는 오류다. 밖으로 그대로 나가면 사람이
// 보는 실패 사유가 「계획을 다시 세운다」 가 된다 — 다시 세우지도 않았는데
// 그렇게 적히면 무엇이 잘못됐는지 알 수 없다(연쇄 작업 둘에서 관측).
//
// 신호를 새로 만들면 여기에 적는다. 적지 않으면 시험이 막는다.
var internalSignals = []error{errRetryPlanning}

// humanReason 은 execute 가 돌려주는 오류에서 안쪽 신호를 걷어 낸다.
//
// 신호가 나는 자리마다 whyKeptRetrying 으로 바꾸는 것이 먼저고, 이것은 그
// 자리를 하나 빠뜨렸을 때를 위한 마지막 관문이다.
func (t *taskContext) humanReason(err error) error {
	if err == nil {
		return nil
	}
	for _, sig := range internalSignals {
		if errors.Is(err, sig) {
			return t.whyKeptRetrying("작업")
		}
	}
	return err
}

// whyKeptRetrying 은 다시 세울 시도가 남지 않았을 때 사람이 볼 사유를 만든다.
// 왜 다시 세우려 했는지는 이미 되먹임에 적혀 있다.
func (t *taskContext) whyKeptRetrying(step string) error {
	why := strings.TrimSpace(t.lastFeedback)
	if why == "" {
		return fmt.Errorf("%s 을(를) 세 번 고쳐 봤지만 끝내 되지 않았다 — 까닭이 기록되지 않았다", step)
	}
	return fmt.Errorf("%s 을(를) 세 번 고쳐 봤지만 끝내 되지 않았다:\n%s", step, clip(why, 1200))
}
