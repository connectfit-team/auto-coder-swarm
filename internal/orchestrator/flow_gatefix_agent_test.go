package orchestrator

import (
	"strings"
	"testing"
)

// 도구를 쥔 고치기가 먼저 서고, 안 되면 기계식으로 물러선다.
// 순서가 뒤집히면 도구는 영영 안 돈다.
func TestAgenticGateFixRunsFirstWithFallback(t *testing.T) {
	src := readSource(t, "flow_gatefix.go")
	agent := strings.Index(src, "t.agenticGateFix()")
	mech := strings.Index(src, "protoTargets(plan)")
	if agent < 0 || mech < 0 {
		t.Fatalf("둘 중 하나가 없다: agent=%d mech=%d", agent, mech)
	}
	if agent > mech {
		t.Error("기계식이 먼저 돈다 — 도구가 영영 안 돈다")
	}
	if !strings.Contains(src, "if t.agenticGateFix() {\n\t\treturn\n\t}") {
		t.Error("도구로 고쳤을 때 기계식을 건너뛰지 않는다")
	}
}

// **모델 말을 믿지 않는다.** 「다 고쳤다」 고 해도 관문을 다시 본다.
func TestAgenticGateFixVerifiesItself(t *testing.T) {
	src := readSource(t, "flow_gatefix_agent.go")
	i := strings.Index(src, "loop.Run(")
	if i < 0 {
		t.Fatal("루프를 안 돌린다")
	}
	after := src[i:]
	if !strings.Contains(after, "t.gateChecks(t.currentDiff())") {
		t.Error("돌린 뒤에 관문을 다시 안 본다 — 모델 말을 그대로 믿는다")
	}
	if !strings.Contains(after, "return false") {
		t.Error("아직 걸리는데 끝냈다고 한다")
	}
}

// 편집은 있던 기계를 쓴다 — #SEARCH/REPLACE 와 자리 찾기 사다리를 버리지 않는다.
func TestEditToolUsesTheExistingCoder(t *testing.T) {
	src := readSource(t, "codertools.go")
	if !strings.Contains(src, "t.coder.ModifyFile(ctx, full, how)") {
		t.Error("edit_file 이 있던 코더를 안 쓴다")
	}
	// 저장소 바깥을 고치면 안 된다.
	if !strings.Contains(src[strings.Index(src, "func (t *taskContext) editViaCoder"):], "insideRepo(") {
		t.Error("edit_file 이 저장소 바깥을 막지 않는다")
	}
}

// 끌 수 있어야 한다 — 나빠지면 되돌릴 길이 있어야 잰다.
func TestAgenticGateFixCanBeTurnedOff(t *testing.T) {
	if !strings.Contains(readSource(t, "flow_gatefix_agent.go"), `os.Getenv("SWARM_GATEFIX_AGENT")`) {
		t.Error("끌 수 있는 길이 없다")
	}
}
