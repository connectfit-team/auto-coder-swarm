package orchestrator

import (
	"strings"
	"testing"
)

// 계약 원본만 고친 것은 아직 아무에게도 보이지 않는다.
// 생성물은 사람이 make push-*apis 를 돌려야 만들어진다.
func TestOnlyContractSources(t *testing.T) {
	protoOnly := `--- a/ceoweb/v1/connect.service.proto
+++ b/ceoweb/v1/connect.service.proto
@@
+  rpc Hold(RequestHold) returns (ResponseHold) {}
`
	if !onlyContractSources(protoOnly) {
		t.Error("계약만 고친 것을 못 알아봤다")
	}
	mixed := protoOnly + `--- a/go/x.go
+++ b/go/x.go
@@
+func A() {}
`
	if onlyContractSources(mixed) {
		t.Error("계약 말고도 고친 것을 계약만이라고 했다")
	}
	if onlyContractSources("") {
		t.Error("빈 diff 를 계약만이라고 했다")
	}
}

// 퍼지는 개수를 묶고, 줄인 사실을 남긴다.
func TestChainReactionIsCapped(t *testing.T) {
	src := readSource(t, "chain_reaction.go")
	for _, must := range []string{
		"onlyContractSources(t.finalDiff)",
		"maxImpactTasks",
		"CHAIN_CAPPED",
		"sort.SliceStable",
	} {
		if !strings.Contains(src, must) {
			t.Errorf("chain_reaction.go 에 %q 가 없다", must)
		}
	}
	if maxImpactTasks > 5 {
		t.Errorf("한 번에 %d 곳까지 퍼진다 — 너무 많다", maxImpactTasks)
	}
}
