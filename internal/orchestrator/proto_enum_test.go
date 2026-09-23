package orchestrator

import (
	"fmt"
	"strings"
	"testing"
)

func TestEnumDigestKeepsHeadAndZero(t *testing.T) {
	src := `enum CEOPolicyType {
  CEO_POLICY_TYPE_UNSPECIFIED = 0;
  CEO_POLICY_TYPE_TERMS_OF_SERVICE = 1;                 // 이용약관 (필수)
  CEO_POLICY_TYPE_PRIVACY_POLICY = 2;
  CEO_POLICY_TYPE_LOCATION_INFO = 3;
  CEO_POLICY_TYPE_SMS = 4;
}`
	got := firstEnumDigest(src)
	if !strings.Contains(got, "CEO_POLICY_TYPE_UNSPECIFIED = 0;") {
		t.Fatalf("0 번이 빠졌다:\n%s", got)
	}
	if strings.Contains(got, "CEO_POLICY_TYPE_SMS") {
		t.Fatalf("세 값만 남겨야 한다:\n%s", got)
	}
	if strings.Contains(got, "이용약관") {
		t.Fatalf("주석까지 끌고 왔다:\n%s", got)
	}
	if !strings.HasSuffix(got, "  ...\n}\n") {
		t.Fatalf("끊었다는 표시가 없다:\n%s", got)
	}
}

func TestEnumDigestEmptyWhenNoEnum(t *testing.T) {
	if got := firstEnumDigest("message A { string b = 1; }"); got != "" {
		t.Fatalf("enum 이 없는데 내놓는다:\n%s", got)
	}
	if got := firstEnumDigest("enum Empty {\n}"); got != "" {
		t.Fatalf("값이 없는 enum 을 본보기로 내놓는다:\n%s", got)
	}
}

// 실제 계약에서 나와야 한다 — 없으면 이 장치는 아무 일도 안 한 것이다.
func TestEnumExampleOnRealContract(t *testing.T) {
	got := protoEnumExample("/home/cnf/cie-repos/proto-ceowebapis",
		"ceoweb/v1/connect.communication.proto")
	if got == "" {
		t.Skip("계약 사본이 없다")
	}
	if !strings.Contains(got, "UNSPECIFIED = 0;") {
		t.Fatalf("0 번 본보기가 없다:\n%s", got)
	}
	if !strings.Contains(got, "0 번에 뜻을 넣으면") {
		t.Fatalf("까닭을 안 적었다:\n%s", got)
	}
	fmt.Println(got)
}
