package orchestrator

import (
	"strings"
	"testing"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
)

// 되돌려야 하는 위반은 여기서 고치지 않는다 — 엉뚱한 것을 고친 것이라
// 계획부터 다시다. 그 뒤에 고칠 수 있는 것이 있으면 그것을 고른다.
func TestFirstFixableSkipsRevertGates(t *testing.T) {
	g, ok := firstFixable([]gateResult{
		{label: "NOT_THE_WORK", revert: true},
		{label: "PROTO_BROKEN"},
	})
	if !ok || g.label != "PROTO_BROKEN" {
		t.Errorf("되돌림 아닌 위반을 골라야 한다: %v %q", ok, g.label)
	}

	if _, ok := firstFixable([]gateResult{{label: "NOTHING_NEW", revert: true}}); ok {
		t.Error("되돌려야 하는 것만 있으면 손대지 않는다")
	}
	if _, ok := firstFixable(nil); ok {
		t.Error("위반이 없으면 고를 것도 없다")
	}
}

func TestProtoTargets(t *testing.T) {
	got := protoTargets(agent.Plan{Changes: []agent.FileChange{
		{FilePath: "web/src/a.ts"},
		{FilePath: "proto/ceo/connect.proto"},
		{FilePath: "proto/ceo/connect.service.proto"},
	}})
	if len(got) != 2 || got[0] != "proto/ceo/connect.proto" {
		t.Errorf("계약 파일만 골라야 한다: %v", got)
	}
}

// 관문을 미리 돌리는 자리는 코딩 **뒤**, 빌드 **앞**이어야 한다.
// 뒤로 밀리면 끝에서 우는 지금과 같아지고, 앞으로 당기면 볼 diff 가 없다.
func TestGateFixRunsBetweenCodingAndBuild(t *testing.T) {
	src := readSource(t, "flow.go")
	exec := strings.Index(src, "t.stepExecution(attempt)")
	fix := strings.Index(src, "t.stepGateFix()")
	verif := strings.Index(src, "t.stepVerification()")
	if exec < 0 || fix < 0 || verif < 0 {
		t.Fatalf("단계가 없다: exec=%d fix=%d verif=%d", exec, fix, verif)
	}
	if !(exec < fix && fix < verif) {
		t.Errorf("차례가 틀렸다: exec=%d fix=%d verif=%d", exec, fix, verif)
	}
}
