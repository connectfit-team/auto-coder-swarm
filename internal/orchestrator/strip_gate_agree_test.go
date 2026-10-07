package orchestrator

import (
	"strings"
	"testing"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
)

// **턴 뒤에 관문이 또 물면 안 된다.**
//
// 기계가 지우는 쪽(agent.StripRestatingComments)과 막는 쪽
// (restatingComments)이 잣대가 다르면, 지우고도 막혀 시도가 날아간다.
// 둘이 같은 것을 보는지 못박는다.
func TestStripAndGateAgree(t *testing.T) {
	src := `service Internal {
  // 내부 서비스에 rpc UpdateReceivedRequestStatus 추가
  rpc UpdateReceivedRequestStatus(RequestUpdateReceivedRequestStatus) returns (ResponseUpdateReceivedRequestStatus) {}
}

// RequestUpdateReceivedRequestStatus 메시지 정의
message RequestUpdateReceivedRequestStatus {
  string request_id = 1;
}

// 보류는 수락도 거절도 아닌 셋째 상태다.
message ReceivedRequest {
  string id = 1;
}
`
	// 기계가 턴다.
	cleaned, n := agent.StripRestatingComments(src)
	if n == 0 {
		t.Fatal("지울 것이 있는데 안 지웠다 — 이 시험의 전제가 틀렸다")
	}

	// 턴 결과를 diff 인 양 넣어 관문에 건다.
	var b strings.Builder
	b.WriteString("+++ b/ceoweb/v1/x.proto\n")
	for _, l := range strings.Split(cleaned, "\n") {
		b.WriteString("+" + l + "\n")
	}
	if bad := restatingComments(b.String()); len(bad) > 0 {
		t.Fatalf("턴 뒤에도 관문이 문다 — 잣대가 어긋났다:\n%s", strings.Join(bad, "\n"))
	}

	// 뜻이 있는 주석은 살아 있어야 한다.
	if !strings.Contains(cleaned, "셋째 상태다") {
		t.Fatalf("뜻이 있는 주석이 사라졌다:\n%s", cleaned)
	}
}
