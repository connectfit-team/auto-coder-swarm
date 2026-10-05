package orchestrator

import (
	"strings"
	"testing"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
)

func targetPlanWith(paths ...string) agent.Plan {
	var p agent.Plan
	for _, f := range paths {
		p.Changes = append(p.Changes, agent.FileChange{FilePath: f})
	}
	return p
}

// 계약은 필드와 RPC 를 다른 파일에 둔다. 둘을 짚어 줬는데 하나만 고르면
// 모델이 한 파일에 둘 다 밀어 넣고 있는 메시지를 또 선언한다(실측 W-70015).
func TestPlanMustCoverEveryPointedFile(t *testing.T) {
	targets := []string{"ceoweb/v1/connect.communication.proto", "ceoweb/v1/connect.service.proto"}

	miss := missingTargetFiles(targets, targetPlanWith("ceoweb/v1/connect.service.proto"))
	if len(miss) != 1 || !strings.Contains(miss[0], "communication") {
		t.Fatalf("빠진 자리를 못 짚었다: %v", miss)
	}

	if miss := missingTargetFiles(targets, targetPlanWith(targets...)); len(miss) != 0 {
		t.Fatalf("둘 다 있는데 빠졌다고 한다: %v", miss)
	}
}

// 경로를 그대로 견주면 안 된다 — 계획이 앞에 뭔가 붙여 오는 일이 있다.
func TestPlanPathsMatchLoosely(t *testing.T) {
	targets := []string{"ceoweb/v1/a.proto", "ceoweb/v1/b.proto"}
	for _, got := range [][]string{
		{"./ceoweb/v1/a.proto", "./ceoweb/v1/b.proto"},
		{"proto-ceowebapis/ceoweb/v1/a.proto", "proto-ceowebapis/ceoweb/v1/b.proto"},
		{"a.proto", "b.proto"},
	} {
		if miss := missingTargetFiles(targets, targetPlanWith(got...)); len(miss) != 0 {
			t.Fatalf("%v 를 못 알아본다: %v", got, miss)
		}
	}
}

// 짚어 준 자리가 하나면 고를 것이 없다 — 아무 말도 하지 않는다.
func TestSingleTargetNeverRejected(t *testing.T) {
	if miss := missingTargetFiles([]string{"a.proto"}, targetPlanWith("b.proto")); len(miss) != 0 {
		t.Fatalf("하나뿐인데 물었다: %v", miss)
	}
	if miss := missingTargetFiles(nil, targetPlanWith("b.proto")); len(miss) != 0 {
		t.Fatalf("짚어 준 것이 없는데 물었다: %v", miss)
	}
}
