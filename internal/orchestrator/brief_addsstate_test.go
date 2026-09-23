package orchestrator

import (
	"strings"
	"testing"
)

// 계약 저장소의 일은 제 일을 남에게 다시 넘기면 안 된다.
//
// 4번이 그 자리였다. (「뜻이 갈리면 사람에게 넘긴다」 는 다른 말이다 — 요청이
// 모호할 때의 이야기라 그대로 둔다.)
func TestAddsStateBriefDoesNotHandOff(t *testing.T) {
	b := NewFeatureBrief("proto-ceowebapis 저장소에 이것을 더해라: 보류 상태",
		"", []string{"ceoweb/v1/connect.communication.proto"}, true)

	if strings.Contains(b, "4. proto 메시지·필드가 필요하면") {
		t.Fatalf("제 일을 넘기라고 시킨다:\n%s", b)
	}
	for _, want := range []string{
		"담을 자리를 만드는 것이 이 일이다",
		"넘길 곳이 여기다",
		"없는 것이 당연하다",
		"같은 이름으로 새로 만들지 않는다",
		"필드 번호는 건드리지 않는다",
	} {
		if !strings.Contains(b, want) {
			t.Fatalf("%q 가 없다:\n%s", want, b)
		}
	}
}

// 소비하는 저장소는 그대로 넘겨야 한다 — 웹에서 계약을 지어내면 안 된다.
func TestConsumerBriefStillHandsOff(t *testing.T) {
	b := NewFeatureBrief("고용주웹에서 연결보류 기능을 추가할거야", "", nil, false)
	if !strings.Contains(b, "4. proto 메시지·필드가 필요하면") {
		t.Fatalf("계약을 지어낼 길이 열렸다:\n%s", b)
	}
	if strings.Contains(b, "넘길 곳이 여기다") {
		t.Fatalf("소비하는 저장소인데 제가 계약을 고치려 한다:\n%s", b)
	}
}
