package agent

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// 첫 답이 산문이어도 다시 물어 받아낸다. 한 번에 죽이면 모델이 흔들린 것과
// 진짜로 못 하는 것이 구별되지 않는다.
func TestCallJSONRetriesUntilParsable(t *testing.T) {
	replies := []string{
		"## 분석 전략\n먼저 파일을 살펴보겠습니다.",
		"```json\n{\"is_feasible\": true}\n```",
	}
	var seen []string
	call := func(p string) (string, error) {
		seen = append(seen, p)
		return replies[len(seen)-1], nil
	}

	var out struct {
		Feasible bool `json:"is_feasible"`
	}
	if _, err := callJSONWith(context.Background(), "전략을 세워라", &out, "Architect", call); err != nil {
		t.Fatalf("두 번째에 성공해야 한다: %v", err)
	}
	if !out.Feasible {
		t.Error("값이 안 담겼다")
	}
	if len(seen) != 2 {
		t.Fatalf("시도 %d회", len(seen))
	}
	if strings.Contains(seen[0], "출력 형식") {
		t.Error("첫 시도부터 잔소리를 붙이면 프롬프트만 길어진다")
	}
	if !strings.Contains(seen[1], "JSON 객체 하나만") {
		t.Error("두 번째 시도에 더 강한 형식 지시가 없다")
	}
}

func TestCallJSONGivesUpWithRawOutput(t *testing.T) {
	replies := []string{"산문", "여전히 산문", "끝까지 산문"}
	n := 0
	call := func(string) (string, error) { n++; return replies[n-1], nil }

	var out map[string]any
	raw, err := callJSONWith(context.Background(), "p", &out, "Architect", call)
	if err == nil {
		t.Fatal("끝내 실패해야 한다")
	}
	if raw != "끝까지 산문" {
		t.Errorf("마지막 원문을 돌려줘야 진단이 된다: %q", raw)
	}
	if n != jsonRetries {
		t.Errorf("시도 %d회, 기대 %d회", n, jsonRetries)
	}
}

// LLM 자체가 실패하면 다시 물어도 같다. 헛돌지 않아야 한다.
func TestCallJSONDoesNotRetryTransportError(t *testing.T) {
	n := 0
	call := func(string) (string, error) { n++; return "", errors.New("연결 끊김") }

	var out map[string]any
	if _, err := callJSONWith(context.Background(), "p", &out, "Architect", call); err == nil {
		t.Fatal("오류를 그대로 올려야 한다")
	}
	if n != 1 {
		t.Errorf("호출 %d회 — 전송 오류에는 재시도하지 않는다", n)
	}
}
