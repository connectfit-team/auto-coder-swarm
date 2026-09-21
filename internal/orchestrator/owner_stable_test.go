package orchestrator

import "testing"

// 「어느 타입에 붙나」 를 물으면 흔들린다 — 한 번은 맞고 한 번은 엉뚱한
// 타입을 골랐다(W-77123 vs W-80412). 고칠 파일에서 바로 따라가면 물을 것이
// 없다.
func TestProtoOwnerViaClientIsStable(t *testing.T) {
	const repo = "/home/cnf/cie-repos/gig_ceo_web"
	// 같은 파일을 여러 번 따라가도 같은 답이 나와야 한다.
	var first string
	for i := 0; i < 3; i++ {
		o, why := protoOwnerViaClient(repo, "src/lib/server/data/connectcud.ts")
		if o == "" {
			t.Skipf("사본이 없거나 모양이 바뀌었다: %s", why)
		}
		if first == "" {
			first = o
		} else if o != first {
			t.Fatalf("같은 파일인데 답이 달라졌다: %q vs %q", first, o)
		}
	}
	if first != "proto-ceowebapis" {
		t.Errorf("임자를 잘못 짚었다: %q", first)
	}
}
