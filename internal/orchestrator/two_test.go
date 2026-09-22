package orchestrator

import (
	"strings"
	"testing"
)

// 필드는 메시지 파일에, RPC 는 서비스 파일에 들어간다. 계약은 그 둘을
// 나눠 두는 일이 흔하다 — 자리를 하나만 알려 주면 한쪽에 다 밀어 넣는다.
func TestHandoffSplitsFieldAndRPCPlaces(t *testing.T) {
	src := readSource(t, "state_chain.go")
	for _, must := range []string{
		"필드: %s 의 %s 메시지",
		"RPC : %s 의 service",
		"그것을 고쳐라 — 같은 이름으로 새로 만들지 마라",
		"protoFileDeclaring(",
	} {
		if !strings.Contains(src, must) {
			t.Errorf("state_chain.go 에 %q 가 없다", must)
		}
	}
	// 파일이 같으면 굳이 둘로 나누지 않는다.
	if !strings.Contains(src, "msgPath != svcPath") {
		t.Error("같은 파일일 때도 둘로 나눠 적는다")
	}
}
