package orchestrator

import (
	"strings"
	"testing"
)

// proto 만 고칠 것이 있는 요청은 PR 이 없다. 그것을 "PR 0개" 로 읽으면
// 올바른 결과가 실패로 보고되고, 사람은 make 를 돌려야 하는 줄 모른다.
func TestProtoOnlyIsNotAFailure(t *testing.T) {
	rs := []VariantResult{{Repo: "proto-userapis", MakeTarget: "push-userapis", Inserted: 2}}

	got := outcomeText(rs)
	if got == "" {
		t.Fatal("결과 글이 비었다 — 부르는 쪽이 실패로 읽는다")
	}
	if !strings.Contains(got, "make push-userapis") {
		t.Fatalf("무엇을 돌려야 하는지 없다: %q", got)
	}
	if h := doneHeadline(rs); !strings.Contains(h, "make") {
		t.Fatalf("머리글이 PR 만 센다: %q", h)
	}
}

// 되돌린 까닭은 마지막 오류 문구까지 살아 있어야 한다.
func TestWhyNoPRKeepsRefusals(t *testing.T) {
	refused := []VariantResult{{
		Repo:        "gig_mobile",
		NeedsManual: []string{"lib/a.dart:10 — 넣으면 문법이 깨져 되돌렸다: 괄호가 안 맞는다"},
	}}
	if got := whyNoPR(refused); !strings.Contains(got, "괄호가 안 맞는다") {
		t.Fatalf("되돌린 까닭이 사라졌다: %q", got)
	}

	failed := []VariantResult{
		{Repo: "a", NeedsManual: []string{"되돌림"}},
		{Repo: "b", Err: "빌드가 깨졌다"},
	}
	if got := whyNoPR(failed); got != "빌드가 깨졌다" {
		t.Fatalf("실패가 먼저 보여야 한다: %q", got)
	}
}
