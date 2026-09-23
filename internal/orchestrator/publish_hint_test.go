package orchestrator

import (
	"strings"
	"testing"
)

// 계약을 고친 일이 승인을 기다리며 멈추면 사람이 볼 다음 걸음이 있어야 한다.
func TestPublishInstructionForContractRepo(t *testing.T) {
	got := howToPublishRepo("proto-ceowebapis")
	for _, want := range []string{"make push-ceowebapis", "protogen", "밀어서 펴내지 않는다", "go get"} {
		if !strings.Contains(got, want) {
			t.Fatalf("%q 가 없다:\n%s", want, got)
		}
	}
	// 생성물을 손으로 건드리는 길도 함께 막아 둬야 한다.
	if !strings.Contains(got, "protoc") || !strings.Contains(got, "*.pb.go") {
		t.Fatalf("생성물을 손으로 고치지 말라는 말이 없다:\n%s", got)
	}
}

// 계약 저장소가 아니면 아무 말도 하지 않는다 — 엉뚱한 안내는 없느니만 못하다.
func TestNoPublishInstructionForOtherRepos(t *testing.T) {
	for _, repo := range []string{"gig_ceo_web", "protogen", "ceo", ""} {
		if got := howToPublishRepo(repo); got != "" {
			t.Fatalf("%s 에 계약 안내가 나간다:\n%s", repo, got)
		}
	}
}
