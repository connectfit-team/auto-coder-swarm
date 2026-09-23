package ckhclient

import (
	"testing"
	"time"
)

// 3분은 모자랐다 — 사내지식의 요약도 코딩 자동화와 같은 모델 하나를 쓴다.
func TestAskWaitDefaultOutlastsSummarization(t *testing.T) {
	t.Setenv("CKH_WAIT_TIMEOUT", "")
	got := askWaitMax()
	if got <= 3*time.Minute {
		t.Fatalf("기본 마감이 아직 짧다: %s", got)
	}
	// CKH 쪽 마감(기본 10분)보다는 짧아야 한다 — 그래야 「실패」 를 받고
	// 끝나지, 이쪽이 먼저 포기해 까닭을 모르는 일이 안 생긴다.
	if got >= 10*time.Minute {
		t.Fatalf("CKH 쪽 마감보다 길거나 같다: %s", got)
	}
}

func TestAskWaitFromEnv(t *testing.T) {
	t.Setenv("CKH_WAIT_TIMEOUT", "30s")
	if got := askWaitMax(); got != 30*time.Second {
		t.Fatalf("환경을 안 읽는다: %s", got)
	}
	for _, bad := range []string{"0", "-1m", "팔분", ""} {
		t.Setenv("CKH_WAIT_TIMEOUT", bad)
		if got := askWaitMax(); got != askWaitDefault {
			t.Fatalf("%q 에서 기본으로 안 돌아간다: %s", bad, got)
		}
	}
}
