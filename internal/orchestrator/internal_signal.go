package orchestrator

import (
	"errors"
	"fmt"
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/gitmgr"
	"github.com/connectfit-team/auto-coder-swarm/internal/insightclient"
	"github.com/connectfit-team/auto-coder-swarm/internal/korean"
)

// internalSignal 은 흐름 안에서만 뜻이 있는 오류다. 밖으로 그대로 나가면
// 사람이 보는 실패 사유가 「계획을 다시 세운다」 가 된다 — 다시 세우지도
// 않았는데 그렇게 적히면 무엇이 잘못됐는지 알 수 없다.
//
// 표식을 값에 붙인다. 손으로 관리하는 목록에 두면 적기를 잊은 신호가 그대로
// 새고, 잘못 적은 신호는 시험만 조용해진다.
type internalSignal struct{ msg string }

func (e *internalSignal) Error() string { return e.msg }

// newInternalSignal 로 만든 오류는 사람에게 그대로 나가지 않는다.
func newInternalSignal(msg string) error { return &internalSignal{msg: msg} }

// 되먹임을 주고 다시 세우면 되는 계획 실패. 남은 시도를 쓴다.
var errRetryPlanning = newInternalSignal("계획을 다시 세운다")

// 문구 자체가 이미 사유인 오류. 관문을 지나도 그대로 둔다.
//
// 여기 적는 것은 「이 문구를 사람이 봐도 된다」 는 뜻이지 관문을 끄는 스위치가
// 아니다 — 안쪽 신호는 표식으로 갈리므로 여기 적어도 가려지지 않는다.
var humanReadableSignals = []error{
	errChangedTestsFailed,
	errNewTypeErrors,
	gitmgr.ErrMisaligned,
	insightclient.ErrAnalysisFailed,
	insightclient.ErrNotAuthorized,
}

// humanReason 은 execute 가 돌려주는 오류에서 안쪽 신호를 걷어 낸다.
//
// 신호가 나는 자리마다 whyKeptRetrying 으로 바꾸는 것이 먼저고, 이것은 그
// 자리를 하나 빠뜨렸을 때를 위한 마지막 관문이다. 몇 번째 시도였는지는
// 여기서 알 수 없으므로 횟수를 말하지 않는다.
func (t *taskContext) humanReason(err error) error {
	if err == nil {
		return nil
	}
	var sig *internalSignal
	if errors.As(err, &sig) {
		return fmt.Errorf("작업이 끝내 되지 않았다.%s", lastFeedbackTail(t.lastFeedback))
	}
	return err
}

// whyKeptRetrying 은 다시 세울 시도가 남지 않았을 때 사람이 볼 사유를 만든다.
// 왜 다시 세우려 했는지는 이미 되먹임에 적혀 있다.
func (t *taskContext) whyKeptRetrying(step string, attempts int) error {
	return fmt.Errorf("%s %d번 고쳐 봤지만 끝내 되지 않았다.%s",
		korean.With(step, "을", "를"), attempts, lastFeedbackTail(t.lastFeedback))
}

// lastFeedbackTail 은 마지막 되먹임을 사유 뒤에 붙인다.
//
// 되먹임은 모델에게 쓴 말이라 「… 다시 계획하라」 같은 지시가 섞여 있다.
// 누구에게 한 말인지 밝히지 않으면 사람은 자기에게 하는 말로 읽는다.
func lastFeedbackTail(feedback string) string {
	why := strings.TrimSpace(feedback)
	if why == "" {
		return " 까닭이 기록되지 않았다."
	}
	return "\n\n[고치던 쪽에 남긴 마지막 되먹임]\n" + clip(why, 1200)
}
