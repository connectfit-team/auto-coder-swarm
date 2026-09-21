package orchestrator

import (
	"strings"
	"testing"
)

// 담을 자리를 만드는 일에는 「담을 자리가 있나」 를 묻지 않는다.
//
// 물으면 스스로 「없다」 고 답하고 다시 남에게 넘긴다 — 연쇄가 한 바퀴 돌고
// 아무것도 안 만든 채 끝난다. 실측으로 자식이 proto-ceowebapis 를 받고도
// 거기서 또 protogen 으로 넘겼다.
func TestStateChainChildBuildsInsteadOfAsking(t *testing.T) {
	flow := readSource(t, "flow.go")
	if !strings.Contains(flow, "!t.req.AddsState && IsNewFeatureRequest") {
		t.Error("담을 자리를 만드는 일에도 담을 자리를 묻는다")
	}
	chain := readSource(t, "state_chain.go")
	if !strings.Contains(chain, "AddsState:    true") {
		t.Error("넘길 때 「이 일이 담을 자리를 만드는 일」 이라는 표시를 안 싣는다")
	}
	types := readSource(t, "types.go")
	if !strings.Contains(types, "AddsState bool") {
		t.Error("요청에 그 표시를 담을 자리가 없다")
	}
}
