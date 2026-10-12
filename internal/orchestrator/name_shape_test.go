package orchestrator

import (
	"os"
	"strings"
	"testing"
)

// 새로 지으라고만 하고 **어떤 이름으로** 지으라는 말이 없었다.
//
// 실측 12판에서 둘 다 이것으로 떨어졌다 — `UpdateRequestHoldStatusRequest`,
// `UpdatePendingStatusRequest`. 관문(#192)이 물어도 세 시도 안에 못 고쳤다.
func TestExampleBlockSaysTheNameShape(t *testing.T) {
	repo := "/home/cnf/cie-repos/proto-ceowebapis"
	if _, err := os.Stat(repo); err != nil {
		t.Skip("계약 사본이 없다")
	}
	b := protoConventionExample(repo, "ceoweb/v1/connect.service.proto", 2)
	if b == "" {
		t.Fatal("본보기가 비었다 — 이 시험이 아무것도 재지 못한다")
	}
	for _, want := range []string{"RequestXxx", "앞에 붙인다", "XxxRequest"} {
		if !strings.Contains(b, want) {
			t.Errorf("%q 가 없다:\n%s", want, b[:min(len(b), 400)])
		}
	}
}

// 막는 자리와 일러 주는 자리가 **같은 셈**을 봐야 어긋나지 않는다.
func TestGateAndHintShareTheCount(t *testing.T) {
	repo := "/home/cnf/cie-repos/proto-ceowebapis"
	if _, err := os.Stat(repo); err != nil {
		t.Skip("계약 사본이 없다")
	}
	pre, suf := nameHabit(repo)
	if pre == 0 {
		t.Fatal("접두형을 하나도 못 셌다")
	}
	line := nameShapeLine(repo)
	if line == "" {
		t.Fatal("접두형이 압도적인데 일러 주지 않는다")
	}
	// 관문이 무는 조건과 같은 문턱을 쓴다.
	if pre < minNameSamples || pre <= suf*nameDominance {
		t.Fatalf("관문은 안 무는데 일러 준다 (접두 %d · 접미 %d)", pre, suf)
	}
}

// 섞어 쓰는 계약에서는 말하지 않는다 — 규칙을 박아 두지 않는다.
func TestNoShapeLineForMixedContract(t *testing.T) {
	repo := protoRepoForTest(t, `syntax = "proto3";
package x.v1;

message GetUserRequest {
  string id = 1;
}
message GetUserResponse {
  string name = 1;
}
`)
	if got := nameShapeLine(repo); got != "" {
		t.Fatalf("접미형 계약에 접두형을 일러 줬다: %s", got)
	}
}

func TestNameHabitWithoutRepo(t *testing.T) {
	if pre, suf := nameHabit(""); pre != 0 || suf != 0 {
		t.Fatalf("빈 경로에서 셌다: %d %d", pre, suf)
	}
	if got := nameShapeLine(""); got != "" {
		t.Fatalf("빈 경로에서 일러 줬다: %s", got)
	}
}
