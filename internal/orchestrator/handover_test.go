package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 승인 대기로 가는 길은 handOver 하나여야 한다.
//
// 길이 셋이었을 때 관문은 한 곳에만 있었고, 실제 사고는 관문이 없는 길로
// 났다(W-76095). 길을 새로 내면 이 시험이 막는다.
func TestOnlyOneDoorToApproval(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		n := strings.Count(string(b), "WaitingApproval: true")
		if n > 0 && f != "handover.go" {
			t.Errorf("%s 에 승인 대기로 가는 길이 %d 개 있다 — handOver 를 거쳐야 한다", f, n)
		}
		total += n
	}
	if total != 1 {
		t.Errorf("승인 대기로 가는 길이 %d 개다 — 하나여야 한다", total)
	}
}

func TestHandOverBlocksToolLeak(t *testing.T) {
	leak := `--- a/src/business/connected_ceo_device_service.go
+++ b/src/business/connected_ceo_device_service.go
@@
+	endpoint := "http://127.0.0.1:8000/v1/chat/completions"
`
	if bad := CheckToolLeak(leak); len(bad) == 0 {
		t.Error("W-76095 의 수정을 통과시켰다")
	}
	ok := `--- a/src/lib/types/connectactions.ts
+++ b/src/lib/types/connectactions.ts
@@
+	HOLD = 'hold',
`
	if bad := CheckToolLeak(ok); len(bad) != 0 {
		t.Errorf("멀쩡한 연결보류 수정을 막았다: %v", bad)
	}
}
