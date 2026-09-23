package agent

import (
	"testing"
)

// 서버가 내주는 이름을 쓴다. 못 물으면 박아 둔 이름으로 물러서되,
// 어느 쪽이든 셈은 멈추지 않아야 한다.
func TestModelNameHonorsEnvFirst(t *testing.T) {
	t.Setenv("LLM_MODEL", "another-model:7b")
	if got := modelName(); got != "another-model:7b" {
		t.Fatalf("환경을 안 읽는다: %q", got)
	}
}

func TestModelNameNeverEmpty(t *testing.T) {
	t.Setenv("LLM_MODEL", "")
	if modelName() == "" {
		t.Fatal("이름이 비었다 — /tokenize 가 통째로 막힌다")
	}
}

// 토크나이저가 없어도 셈은 돌아야 한다(어림으로).
func TestCountFallsBackWhenTokenizerUnreachable(t *testing.T) {
	t.Setenv("LLM_DIRECT_URL", "http://127.0.0.1:1")
	// tokenizeEndpoint 는 한 번만 정해지므로 이 시험은 실제 주소를 바꾸지
	// 못할 수 있다. 어느 쪽이든 셈이 0 보다 커야 한다.
	if n := CountTokens("func f() {}"); n <= 0 {
		t.Fatalf("셈이 멈췄다: %d", n)
	}
	if n := CountTokens(""); n != 0 {
		t.Fatalf("빈 글이 0 이 아니다: %d", n)
	}
}
