package orchestrator

import "testing"

// 계획이 화면 파일만 짚은 회차에서는 gRPC 를 부르는 곳이 없어 고리가 끊겼다.
// 화면은 데이터 모듈을 들여오고, 그 모듈이 공장을 부른다.
func TestProtoContractsDeep(t *testing.T) {
	const repo = "/home/cnf/cie-repos/gig_ceo_web"
	if got, _ := protoContractsDeep(repo, "src/lib/server/data/connectcud.ts", 2); len(got) == 0 || got[0].owner != "proto-ceowebapis" {
		t.Skip("사본이 없거나 모양이 바뀌었다")
	}
	got, why := protoContractsDeep(repo, "src/routes/(auth)/store/connect/+page.server.ts", 2)
	if len(got) == 0 || got[0].owner != "proto-ceowebapis" {
		t.Errorf("화면 파일에서 못 닿았다: %v (%s)", got, why)
	}
}

// 한 파일이 계약 둘을 쓰면 둘 다 나와야 한다. 첫 공장 하나만 보면 어느
// 계약을 고칠지가 파일 안 글자 순서로 정해진다.
func TestProtoContractsInFileFindsEveryFactory(t *testing.T) {
	const repo = "/home/cnf/cie-repos/gig_ceo_web"
	got, why := protoContractsInFile(repo, "src/lib/server/data/subscription.ts")
	if len(got) == 0 {
		t.Skipf("사본이 없거나 모양이 바뀌었다: %s", why)
	}
	if len(got) < 2 {
		t.Errorf("계약 둘을 쓰는 파일에서 %d개만 나왔다: %v", len(got), got)
	}
	seen := map[string]bool{}
	for _, c := range got {
		if seen[c.contract] {
			t.Errorf("같은 계약이 두 번 나왔다: %s", c.contract)
		}
		seen[c.contract] = true
	}
}
