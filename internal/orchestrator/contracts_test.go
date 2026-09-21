package orchestrator

import "testing"

// 한 저장소가 계약을 여럿 펴낸다. 저장소로 묶으면 뒤의 것이 앞의 것을
// 덮어써, 어느 계약을 고칠지가 훑는 순서로 정해진다.
func TestTraceContractsKeepsEachContract(t *testing.T) {
	const ceo = "src/lib/server/protos/ceowebapis/ceoweb/v1/ceo.service.ts"
	const connect = "src/lib/server/protos/ceowebapis/ceoweb/v1/connect.service.ts"
	trace := func(f string) (string, string, string) {
		switch f {
		case "staff.ts":
			return "proto-ceowebapis", ceo, "getStaffClient() → StaffInternalDefinition → " + ceo
		case "connect.ts":
			return "proto-ceowebapis", connect, "getConnectClient() → ConnectCEOWebDefinition → " + connect
		case "another.ts": // 같은 계약에 또 닿는다
			return "proto-ceowebapis", connect, "getConnectClient() → ConnectCEOWebDefinition → " + connect
		}
		return "", "", "gRPC 공장을 부르지 않는다"
	}

	order, seen := traceContracts([]string{"staff.ts", "connect.ts", "another.ts", "page.svelte"}, trace)
	if len(order) != 2 {
		t.Fatalf("계약 2개여야 한다 — %d개: %v", len(order), order)
	}
	if order[0] != ceo || order[1] != connect {
		t.Errorf("처음 본 순서를 잃었다: %v", order)
	}
	if seen[connect].contract != connect {
		t.Errorf("계약 경로를 값으로 들고 있지 않다: %q", seen[connect].contract)
	}
	if got := seen[connect].why; len(got) < 10 || got[:10] != "connect.ts" {
		t.Errorf("근거에 어느 파일에서 왔는지가 없다: %q", got)
	}
}

// 생성물 경로를 못 읽으면 저장소로 묶는다 — 그래도 후보는 남아야 한다.
func TestTraceContractsFallsBackToRepo(t *testing.T) {
	trace := func(f string) (string, string, string) {
		return "some-repo", "", "공장은 찾았지만 경로를 못 읽었다"
	}
	order, seen := traceContracts([]string{"a.ts", "b.ts"}, trace)
	if len(order) != 1 || order[0] != "some-repo" {
		t.Fatalf("저장소 하나로 묶여야 한다: %v", order)
	}
	if seen["some-repo"].contract != "" {
		t.Errorf("없는 계약 경로를 지어냈다: %q", seen["some-repo"].contract)
	}
}
