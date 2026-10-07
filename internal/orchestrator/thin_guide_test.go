package orchestrator

import (
	"strings"
	"testing"
)

// 안내문이 통째로 빈 채로 계약을 쓰면 말한다.
//
// 실측: `connect.service.proto` 를 고치는 코더가 안내문을 **44자** 받았고
// 여섯 블록 가운데 하나도 없었다. 그러고도 아무 말이 없었다.
func TestThinGuideIsNamed(t *testing.T) {
	got := missingProtoGuides("ceoweb/v1/connect.service.proto", "이 파일에 보류 상태를 바꾸는 rpc 를 더해라")
	if got == "" {
		t.Fatal("본보기가 하나도 없는데 아무 말이 없다")
	}
	for _, want := range []string{"상태 타입", "service", "enum"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q 가 빠진 것에 안 적혔다: %s", want, got)
		}
	}
}

// 하나라도 있으면 말하지 않는다. 모든 파일에 셋이 다 맞는 것은 아니다 —
// service 가 없는 파일에 service 예시는 못 만든다.
func TestOneGuideIsEnoughToStaySilent(t *testing.T) {
	instr := "[이 계약이 상태를 담는 꼴 — 그대로 따라라]\n  int32 status = 3;\n"
	if got := missingProtoGuides("x.proto", instr); got != "" {
		t.Fatalf("하나는 있는데 말했다: %s", got)
	}
}

// 계약이 아닌 파일에는 해당 없다.
func TestNonProtoIsNotChecked(t *testing.T) {
	if got := missingProtoGuides("internal/handler/x.go", ""); got != "" {
		t.Fatalf("계약이 아닌데 말했다: %s", got)
	}
}

// 글머리가 바뀌면 이 검사가 조용히 꺼진다 — 진짜 안내문으로 맞춰 둔다.
func TestGuideMarksMatchTheRealText(t *testing.T) {
	repo := "/home/cnf/cie-repos/proto-ceowebapis"
	instr := protoConventionExample(repo, "ceoweb/v1/connect.service.proto", 2)
	if instr == "" {
		t.Skip("계약 사본이 없다")
	}
	if got := missingProtoGuides("ceoweb/v1/connect.service.proto", instr); got != "" {
		t.Fatalf("진짜 안내문인데 빠졌다고 한다 (글머리가 어긋났다): %s\n%s", got, instr[:min(len(instr), 400)])
	}
}
