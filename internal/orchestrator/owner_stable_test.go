package orchestrator

import "testing"

// 「어느 타입에 붙나」 를 물으면 흔들린다. 고칠 파일에서 바로 따라가면
// 같은 답이 나온다.
func TestProtoContractsInFileIsStable(t *testing.T) {
	const repo = "/home/cnf/cie-repos/gig_ceo_web"
	var first string
	for i := 0; i < 3; i++ {
		got, why := protoContractsInFile(repo, "src/lib/server/data/connectcud.ts")
		if len(got) == 0 {
			t.Skipf("사본이 없거나 모양이 바뀌었다: %s", why)
		}
		if first == "" {
			first = got[0].owner
		} else if got[0].owner != first {
			t.Fatalf("같은 파일인데 답이 달라졌다: %q vs %q", first, got[0].owner)
		}
	}
	if first != "proto-ceowebapis" {
		t.Errorf("임자를 잘못 짚었다: %q", first)
	}
}
