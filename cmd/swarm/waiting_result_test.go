package main

import (
	"os"
	"strings"
	"testing"
)

// 승인 대기로 바꿀 때 결과를 버리면 안 된다.
//
// 여기서 "" 를 넘기고 있었다. 그래서 계약 저장소 일이 승인을 기다리며 멈출 때
// 화면에 다음 걸음이 비어 있었다 — 이 저장소는 밀어서 펴내는 것이 아니라
// protogen 에서 make push-*apis 로 펴낸다는 사실이 어디에도 안 보였다
// (실측 W-13896). 흐름 전체를 돌리지 않고 그 줄이 남아 있는지만 본다.
func TestWaitingApprovalKeepsResult(t *testing.T) {
	b, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("main.go 를 못 읽었다: %v", err)
	}
	src := string(b)

	i := strings.Index(src, "res.WaitingApproval")
	if i < 0 {
		t.Fatal("승인 대기 갈래를 못 찾았다 — 이 시험이 지키려던 자리가 사라졌다")
	}
	seg := src[i:]
	if j := strings.Index(seg, "} else {"); j > 0 {
		seg = seg[:j]
	}
	if !strings.Contains(seg, "StatusWaitingApproval, res.Result") {
		t.Fatalf("결과를 버린다:\n%s", seg)
	}
}
