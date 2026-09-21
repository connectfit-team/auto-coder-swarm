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
