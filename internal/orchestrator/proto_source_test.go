package orchestrator

import (
	"strings"
	"testing"
)

// 계약 경로는 까닭 글에서 긁어내지 않고 값으로 들고 다닌다.
//
// 글에서 뽑으면 고칠 파일이 생성물일 때 그 파일이 계약 자리에 들어간다.
func TestTraceCarriesContractPathAsValue(t *testing.T) {
	const gen = "src/lib/server/protos/ceowebapis/ceoweb/v1/connect.service.ts"
	trace := func(f string) []tracedContract {
		return []tracedContract{{owner: "proto-ceowebapis", contract: gen, why: "getConnectClient() → ConnectCEOWebDefinition → " + gen}}
	}
	// 고칠 파일 자체가 생성물이어도 계약은 흔들리지 않는다.
	_, seen := traceContracts([]string{"src/lib/server/protos/userapis/user/v1/service.ts"}, trace)
	c, ok := seen[gen]
	if !ok {
		t.Fatalf("계약을 못 담았다: %v", seen)
	}
	if c.contract != gen {
		t.Errorf("계약 경로가 흔들렸다: %q", c.contract)
	}
	if !strings.Contains(c.why, "protos/userapis") {
		t.Errorf("어느 파일에서 왔는지가 빠졌다: %q", c.why)
	}
}
