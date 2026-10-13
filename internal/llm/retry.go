package llm

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// 연결이 끊겼을 때 기다리는 시간. 5초에서 시작해 두 배씩, 60초에서 멈춘다.
// codex 가 같은 자리에 쓰는 값이다(responses_retry.rs).
const (
	initialRetryDelay = 5 * time.Second
	maxRetryDelay     = 60 * time.Second
	defaultMaxRetries = 4
)

// maxRetries 는 몇 번까지 다시 걸어 볼지다. 0 이면 재시도하지 않는다.
func maxRetries() int {
	if v := strings.TrimSpace(os.Getenv("LLM_MAX_RETRIES")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
	}
	return defaultMaxRetries
}

// worthRetrying 은 다시 걸어 볼 값어치가 있는 실패인지 본다.
//
// **끝난 오류는 다시 걸지 않는다.** 400 은 대개 창 길이 문제라 그대로 다시
// 보내면 또 400 이고, 기다린 만큼만 버린다. 연결이 끊긴 것과 서버가 5xx 를
// 낸 것만 다시 건다 — 이 기계는 하루 세 번 재부팅하고(05:00·13:00·21:00)
// vLLM 이 뜨는 동안 호출이 떨어진다.
func worthRetrying(status int, err error) bool {
	if err != nil {
		// 전송이 안 됐다 — 답을 못 받았으니 다시 보내도 같은 일이 두 번 일어나지 않는다.
		return true
	}
	return status == http.StatusTooManyRequests || status >= 500
}

// waitBefore 는 다음 시도까지 기다린다. 멈추라고 하면 바로 돌아온다.
func waitBefore(ctx context.Context, attempt int, what string) bool {
	d := initialRetryDelay
	for i := 1; i < attempt; i++ {
		d *= 2
		if d >= maxRetryDelay {
			d = maxRetryDelay
			break
		}
	}
	log.Printf("[LLM] %s — %v 뒤에 다시 건다 (%d/%d)", what, d, attempt, maxRetries())
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}
