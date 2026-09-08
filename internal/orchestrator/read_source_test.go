package orchestrator

import (
	"os"
	"testing"
)

// readSource 는 이 꾸러미의 소스를 읽는다. 흐름 전체를 돌리지 않고 규칙이
// 코드에 남아 있는지 보는 데 쓴다 — 다음 사람이 그 줄을 지우면 걸린다.
func readSource(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("%s 를 읽지 못했다: %v", name, err)
	}
	return string(b)
}
