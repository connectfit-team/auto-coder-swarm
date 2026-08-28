package orchestrator

import (
	"context"
	"fmt"
	"log"
	"time"
	"unicode/utf8"
)

func (o *SwarmOrchestrator) logDeepTechnical(ctx context.Context, taskID string, stage, message, prompt, rawResult string) {
	log.Printf("[%s] [%s] %s", taskID, stage, message)

	if o.store != nil {
		// **원본 응답을 버리지 않는다.**
		//
		// 여기는 오래 `""` 를 넘겼다(속도 때문에 껐다고 적혀 있었다). 그래서
		// rawResult 를 받아 놓고 통째로 버렸고, "왜 거절됐나"·"모델이 뭘 뱉었나"
		// 를 남기려고 부른 모든 자리가 **아무것도 남기지 않았다.** 실제로
		// 비평가 거절 이유를 찾다가 로그가 비어 있어 한참 헤맸다.
		//
		// 길이만 묶으면 된다 — 프롬프트·응답 전문이 아니라 진단에 필요한 앞부분이다.
		o.store.AddDeepLog(taskID, stage, message, clipLog(prompt), clipLog(rawResult))
		task, _ := o.store.GetTaskByID(taskID)
		if task != nil {
			newState := fmt.Sprintf("%s\n[%s] %s: %s", task.ContextState, time.Now().Format("15:04:05"), stage, message)
			o.store.UpdateContextState(taskID, newState)
		}
	}
}

// 로그 한 칸의 상한. 진단에는 앞부분이면 충분하고, 전문을 넣으면 DB 가 커진다.
const maxLogChars = 4000

func clipLog(s string) string {
	if len(s) <= maxLogChars {
		return s
	}
	s = s[:maxLogChars]
	for len(s) > 0 && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s + "\n… (생략)"
}
