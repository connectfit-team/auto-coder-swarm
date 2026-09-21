package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fakeRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// 계약 목록은 계획이 무엇을 짚었든 같아야 한다 — 저장소에 다 적혀 있다.
func TestRepoContractsFindsEveryFactory(t *testing.T) {
	root := fakeRepo(t, map[string]string{
		"src/lib/server/protos/ceowebapis/ceoweb/v1/connect.service.ts": "export const ConnectCEOWebDefinition = {}\n",
		"src/lib/server/protos/ceowebapis/ceoweb/v1/ceo.service.ts":     "export const StaffInternalDefinition = {}\n",
		"src/lib/server/protos/purchaseapis/purchase/v1/service.ts":     "export const PurchaseDefinition = {}\n",
		"src/lib/server/grpc/clients.ts": `
import { ConnectCEOWebDefinition } from "../protos/ceowebapis/ceoweb/v1/connect.service";
import { StaffInternalDefinition } from "../protos/ceowebapis/ceoweb/v1/ceo.service";
import { PurchaseDefinition } from "../protos/purchaseapis/purchase/v1/service";

export const getConnectClient = createGuardedClient<typeof ConnectCEOWebDefinition, C>(x);

// 앱과 같은 RPC 다. 조회에는 쓰지 마라 — 읽기 전용이다.
export const getStaffClient = createGuardedClient<
	typeof StaffInternalDefinition, S>(x);

export const getPurchaseClient = createClient(PurchaseDefinition, y);
`,
		// 훑지 않아야 하는 곳
		"node_modules/junk/clients.ts": `
import { FakeDefinition } from "../protos/fakeapis/v1/service";
export const getFakeClient = createGuardedClient<typeof FakeDefinition, F>(x);
`,
	})

	scan := repoContracts(root)
	if len(scan.contracts) != 3 {
		t.Fatalf("계약 3개여야 한다 — %d개: %v", len(scan.contracts), keysOf(scan.contracts))
	}
	if scan.truncated || scan.note() != "" {
		t.Errorf("온전히 훑었는데 말이 붙었다: %q", scan.note())
	}
	connect := "src/lib/server/protos/ceowebapis/ceoweb/v1/connect.service.ts"
	ceo := "src/lib/server/protos/ceowebapis/ceoweb/v1/ceo.service.ts"
	purchase := "src/lib/server/protos/purchaseapis/purchase/v1/service.ts"
	for _, want := range []string{connect, ceo, purchase} {
		if _, ok := scan.contracts[want]; !ok {
			t.Errorf("%s 가 빠졌다", want)
		}
	}
	if got := scan.contracts[connect].owner; got != "proto-ceowebapis" {
		t.Errorf("임자가 틀렸다: %q", got)
	}
	// 줄바꿈된 제네릭 선언도 잡고, 그 위에 달린 경고를 함께 싣는다.
	if got := scan.contracts[ceo].note; !strings.Contains(got, "조회에는 쓰지 마라") {
		t.Errorf("계약에 적힌 경고가 빠졌다: %q", got)
	}
	// createClient(Def, …) 꼴도 같은 계약이다.
	if got := scan.contracts[purchase].why; !strings.Contains(got, "getPurchaseClient()") {
		t.Errorf("다른 공장 관행을 못 읽었다: %q", got)
	}
	// 계약 경로는 값으로 들고 다닌다.
	if got := scan.contracts[connect].contract; got != connect {
		t.Errorf("계약 경로가 비었다: %q", got)
	}
}

// 못 읽었으면 못 읽었다고 해야 한다. 조용한 0개는 「이 저장소에 없다」 로 읽힌다.
func TestRepoContractsSaysWhyItFoundNothing(t *testing.T) {
	if got := repoContracts("").note(); got == "" {
		t.Error("경로가 없는데 아무 말이 없다")
	}
	empty := repoContracts(t.TempDir())
	if len(empty.contracts) != 0 || empty.note() == "" {
		t.Errorf("빈 저장소에 말이 없다: %q", empty.note())
	}

	// 공장은 있는데 들여오기 관행을 못 읽는 저장소
	root := fakeRepo(t, map[string]string{
		"src/lib/server/grpc/service.ts": `
import { PurchaseServiceDefinition } from "@@unknown/purchase";
export const getPurchaseClient = createClient(PurchaseServiceDefinition, y);
`,
	})
	scan := repoContracts(root)
	if len(scan.contracts) != 0 {
		t.Fatalf("계약이 나오면 안 된다: %v", keysOf(scan.contracts))
	}
	if scan.factories == 0 {
		t.Error("공장 정의를 못 봤다")
	}
	if !strings.Contains(scan.note(), "들여오기 관행") {
		t.Errorf("까닭이 틀렸다: %q", scan.note())
	}
}

// @/ · ~/ · src/ 별칭도 푼다.
func TestRepoContractsResolvesCommonAliases(t *testing.T) {
	root := fakeRepo(t, map[string]string{
		"src/protos/userapis/user/v1/service.ts": "export const UserServiceDefinition = {}\n",
		"src/grpc/clients.ts": `
import { UserServiceDefinition } from "@/protos/userapis/user/v1/service";
export const getUserClient = createClient(UserServiceDefinition, y);
`,
	})
	scan := repoContracts(root)
	if len(scan.contracts) != 1 {
		t.Fatalf("별칭을 못 풀었다: %v — %s", keysOf(scan.contracts), scan.note())
	}
	if got := scan.contracts["src/protos/userapis/user/v1/service.ts"].owner; got != "proto-userapis" {
		t.Errorf("임자가 틀렸다: %q", got)
	}
}

func keysOf(m map[string]tracedContract) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
