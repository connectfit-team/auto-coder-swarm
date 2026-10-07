package orchestrator

import (
	"os"
	"strings"
	"testing"
)

const targetSampleProto = `syntax = "proto3";

message Other {
  string a = 1;
}

message ReceivedRequest {
  string request_id = 1;
  // 보낸 사람
  string sender_id = 2;
  message Inner {
    string deep = 77;
  }
  int32 kind = 5;
}

message After {
  string z = 1;
}
`

func TestMessageBlockTakesOnlyThatMessage(t *testing.T) {
	block, next := messageBlock(targetSampleProto, "ReceivedRequest")
	if !strings.HasPrefix(block, "message ReceivedRequest {") {
		t.Fatalf("덩어리가 그 메시지에서 시작하지 않는다:\n%s", block)
	}
	if strings.Contains(block, "message After") || strings.Contains(block, "message Other") {
		t.Fatalf("이웃 메시지가 섞였다:\n%s", block)
	}
	if !strings.HasSuffix(block, "}") {
		t.Fatalf("닫는 괄호로 끝나지 않는다:\n%s", block)
	}
	// 속 메시지의 77 을 세면 다음 번호가 78 이 된다 — 깊이 1 만 본다.
	if next != 6 {
		t.Fatalf("다음 빈 번호가 6 이어야 하는데 %d 다", next)
	}
}

func TestMessageBlockUnknownMessage(t *testing.T) {
	if block, _ := messageBlock(targetSampleProto, "NoSuchMessage"); block != "" {
		t.Fatalf("없는 메시지에 덩어리를 냈다: %q", block)
	}
}

func TestMessageBlockUnclosedGivesNothing(t *testing.T) {
	// 반쪽을 보여 주면 자식이 그 모양을 따라 쓴다.
	if block, _ := messageBlock("message Half {\n  string a = 1;\n", "Half"); block != "" {
		t.Fatalf("닫히지 않은 메시지를 보여 줬다: %q", block)
	}
}

func TestMessageBlockLongMessageKeepsTail(t *testing.T) {
	var b strings.Builder
	b.WriteString("message Big {\n")
	for i := 1; i <= 60; i++ {
		b.WriteString("  string f = ")
		b.WriteString(strings.Repeat("", 0))
		b.WriteString(numToStr(i))
		b.WriteString(";\n")
	}
	b.WriteString("}\n")
	block, next := messageBlock(b.String(), "Big")
	if next != 61 {
		t.Fatalf("다음 번호가 61 이어야 하는데 %d 다", next)
	}
	if !strings.Contains(block, "줄 줄임") {
		t.Fatalf("긴 메시지를 줄이지 않았다")
	}
	if !strings.Contains(block, "= 60;") || !strings.HasSuffix(block, "}") {
		t.Fatalf("꼬리(큰 번호와 닫는 괄호)가 남지 않았다:\n%s", block)
	}
}

func numToStr(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	return string(d)
}

// 진짜 계약에 대고 — 자식이 되풀이해 틀린 그 메시지다.
func TestTargetMessageBodyOnRealContract(t *testing.T) {
	repo := "/home/cnf/cie-repos/proto-ceowebapis"
	if _, err := os.Stat(repo); err != nil {
		t.Skip("계약 사본이 없다")
	}
	out := targetMessageBody(repo, "ReceivedRequest")
	if out == "" {
		t.Skip("ReceivedRequest 가 이 사본에 없다")
	}
	for _, want := range []string{"ReceivedRequest", ".proto", "다음 빈 번호", "```proto"} {
		if !strings.Contains(out, want) {
			t.Fatalf("%q 가 없다:\n%s", want, out)
		}
	}
	t.Logf("\n%s", out)
}
