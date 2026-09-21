package orchestrator

import (
	"os"
	"path/filepath"
	"testing"
)

// 계약 목록은 계획이 무엇을 짚었든 같아야 한다 — 저장소에 다 적혀 있다.
func TestRepoContractsFindsEveryFactory(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("src/lib/server/protos/ceowebapis/ceoweb/v1/connect.service.ts", "export const ConnectCEOWebDefinition = {}\n")
	write("src/lib/server/protos/ceowebapis/ceoweb/v1/ceo.service.ts", "export const StaffInternalDefinition = {}\n")
	write("src/lib/server/protos/purchaseapis/purchase/v1/service.ts", "export const PurchaseDefinition = {}\n")
	write("src/lib/server/grpc/clients.ts", `
import { ConnectCEOWebDefinition } from "../protos/ceowebapis/ceoweb/v1/connect.service";
import { StaffInternalDefinition } from "../protos/ceowebapis/ceoweb/v1/ceo.service";
import { PurchaseDefinition } from "../protos/purchaseapis/purchase/v1/service";

export const getConnectClient = createGuardedClient<typeof ConnectCEOWebDefinition, C>(x);
export const getStaffClient = createGuardedClient<typeof StaffInternalDefinition, S>(x);
export const getPurchaseClient = createGuardedClient<typeof PurchaseDefinition, P>(x);
`)
	// 훑지 않아야 하는 곳
	write("node_modules/junk/clients.ts", `
import { FakeDefinition } from "../protos/fakeapis/v1/service";
export const getFakeClient = createGuardedClient<typeof FakeDefinition, F>(x);
`)

	got := repoContracts(root)
	if len(got) != 3 {
		t.Fatalf("계약 3개여야 한다 — %d개: %v", len(got), keysOf(got))
	}
	for _, want := range []string{
		"src/lib/server/protos/ceowebapis/ceoweb/v1/connect.service.ts",
		"src/lib/server/protos/ceowebapis/ceoweb/v1/ceo.service.ts",
		"src/lib/server/protos/purchaseapis/purchase/v1/service.ts",
	} {
		if _, ok := got[want]; !ok {
			t.Errorf("%s 가 빠졌다", want)
		}
	}
	if c := got["src/lib/server/protos/ceowebapis/ceoweb/v1/connect.service.ts"]; c.owner != "proto-ceowebapis" {
		t.Errorf("임자가 틀렸다: %q", c.owner)
	}
	if c := got["src/lib/server/protos/purchaseapis/purchase/v1/service.ts"]; c.owner != "proto-purchaseapis" {
		t.Errorf("임자가 틀렸다: %q", c.owner)
	}
}

func TestRepoContractsHandlesEmptyRepo(t *testing.T) {
	if got := repoContracts(""); len(got) != 0 {
		t.Errorf("경로가 없으면 빈 목록이어야 한다: %v", keysOf(got))
	}
	if got := repoContracts(t.TempDir()); len(got) != 0 {
		t.Errorf("빈 저장소면 빈 목록이어야 한다: %v", keysOf(got))
	}
}

func keysOf(m map[string]tracedContract) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// 고른 뒤 근거 문자열에 표와 득표가 덧붙는다. 그래도 계약 경로를 바르게
// 뽑아야 한다 — 여기서 어긋나면 엉뚱한 .proto 를 고치라고 넘긴다.
func TestComposedWhyStillYieldsContractPath(t *testing.T) {
	want := "src/lib/server/protos/ceowebapis/ceoweb/v1/connect.service.ts"
	why := "getConnectClient() → ConnectCEOWebDefinition → " + want +
		" · 계약 후보 17 가운데 " + want + " 를 골랐다(3/3표)"
	if got := lastProtosPath(why); got != want {
		t.Errorf("계약 경로를 못 뽑았다: %q", got)
	}
	traced := "connect.ts → getConnectClient() → ConnectCEOWebDefinition → " + want +
		" · 고르지 못해 따라간 것을 쓴다"
	if got := lastProtosPath(traced); got != want {
		t.Errorf("따라간 쪽에서 경로를 못 뽑았다: %q", got)
	}
}
