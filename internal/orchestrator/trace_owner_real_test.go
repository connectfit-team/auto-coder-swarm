package orchestrator

import (
	"strings"
	"testing"
)

// 손으로 쓴 타입이어도 그 데이터가 어디서 오는지는 파일에 적혀 있다.
//
//	ReceivedRequest → getConnectClient() → ConnectCEOWebDefinition
//	  → …/protos/ceowebapis/… → proto-ceowebapis
func TestProtoContractsOnRealRepo(t *testing.T) {
	const repo = "/home/cnf/cie-repos/gig_ceo_web"
	got, why := protoContractsInFile(repo, "src/lib/server/data/connectcud.ts")
	if len(got) == 0 {
		t.Skipf("사본이 없거나 모양이 바뀌었다: %s", why)
	}
	c := got[0]
	if c.owner != "proto-ceowebapis" {
		t.Errorf("임자를 잘못 짚었다: %q (%s)", c.owner, c.why)
	}
	if !strings.Contains(c.contract, "/protos/") || !strings.HasSuffix(c.contract, ".ts") {
		t.Errorf("생성물 경로를 값으로 안 돌려준다: %q", c.contract)
	}
	for _, want := range []string{"getConnectClient", "Definition", "protos/"} {
		if !strings.Contains(c.why, want) {
			t.Errorf("까닭에 %q 가 없다: %s", want, c.why)
		}
	}
}
