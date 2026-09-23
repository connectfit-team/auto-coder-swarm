package orchestrator

import (
	"strings"
	"testing"
)

// 이어진 일의 요청문은 **우리가** 쓴다. 그 낱말이 newFeatureRe 에 들어 있는지에
// 계약 수정이 통째로 달려 있으면 안 된다 — 실제로 「더해라」 가 없어 죽었다.
func TestChainRequestIsNotClassifiedByWording(t *testing.T) {
	req := "proto-ceowebapis 저장소에 이것을 더해라: 연결 요청에 보류 상태를 담을 필드"
	byWording := IsNewFeatureRequest(req)
	byFlag := StatelessRequest{UserRequest: req, AddsState: true}.AddsState
	if !byFlag {
		t.Fatal("AddsState 가 서지 않았다")
	}
	if !byWording {
		t.Log("낱말로는 못 맞힌다 — AddsState 가 받친다 (이것이 이 변경의 까닭이다)")
	}
}

// 분석이 "확인하지 못했다" 고 해도, 담을 자리를 만드는 일이면 멈추면 안 된다.
func TestAddsStateSurvivesNotFound(t *testing.T) {
	analysis := strings.Join([]string{
		"`connect.communication.proto` 파일의 전체 LOC 는 확인하지 못했습니다.",
		"`ReceivedRequest` 메시지에 이미 보류 상태를 담는 필드가 존재하는지 확인하지 못했습니다.",
		"`ReceivedRequest` 메시지에 보류 상태를 담을 필드를 추가할 수 있습니다.",
	}, "\n")

	if _, notFound := AnalysisSaysNotFound(analysis); !notFound {
		t.Fatal("이 글은 「못 찾았다」 로 읽힌다 — 전제가 바뀌었다")
	}

	req := StatelessRequest{
		UserRequest: "proto-ceowebapis 저장소에 이것을 더해라: 연결 보류 상태",
		AddsState:   true,
	}
	if !(req.AddsState || IsNewFeatureRequest(req.UserRequest)) {
		t.Fatal("담을 자리를 만드는 일인데 「고칠 것이 없다」 로 멈춘다")
	}
}

// 반대쪽 — 고장 수리인데 눈이 못 찾았다고 하면 여전히 멈춰야 한다.
func TestPlainFixStillStops(t *testing.T) {
	req := StatelessRequest{UserRequest: "연결 목록이 안 보이는 버그를 고쳐라"}
	if req.AddsState || IsNewFeatureRequest(req.UserRequest) {
		t.Fatal("고장 수리인데 새로 만드는 일로 샌다 — 없는 것을 지어내게 된다")
	}
}
