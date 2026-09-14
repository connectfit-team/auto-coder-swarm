package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func protoFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	w := func(p, s string) {
		full := filepath.Join(dir, p)
		os.MkdirAll(filepath.Dir(full), 0o755)
		if err := os.WriteFile(full, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// 생성물 — 손으로 고치지 않는다. 필드가 없으면 proto 저장소의 일이다.
	w("src/lib/server/protos/ceowebapis/ceoweb/v1/connect.communication.ts",
		"export interface ConnectableStaff { workplaceId: string; }\n")
	// 이 저장소가 손으로 쓴 타입 — 여기서 고친다.
	w("src/lib/types/row.ts", "export type BulkRow = { blockedReason: string };\n")
	return dir
}

func TestSplitMissing(t *testing.T) {
	dir := protoFixture(t)
	contract, local := splitMissing(dir, []string{
		"ConnectableStaff.hold", // 생성물 → proto-ceowebapis
		"BulkRow.status",        // 이 저장소가 쓴 타입 → 여기서 고친다
		"pending",               // 그냥 없는 이름 → 이 저장소의 실수
	})

	if got := contract["proto-ceowebapis"]; len(got) != 1 || got[0] != "ConnectableStaff.hold" {
		t.Errorf("계약 구멍을 못 갈랐다: %v", contract)
	}
	if strings.Join(local, ",") != "BulkRow.status,pending" {
		t.Errorf("이 저장소의 실수를 못 갈랐다: %v", local)
	}
}

func TestProtoOwnerRepo(t *testing.T) {
	cases := map[string]string{
		"src/lib/server/protos/ceowebapis/ceoweb/v1/message.ts":         "proto-ceowebapis",
		"src/lib/server/protos/laborcontractapis/laborcontract/v1/m.ts": "proto-laborcontractapis",
		"src/lib/types/session.ts":                                      "",
		"src/lib/server/protos/proto-userapis/user/v1/message.ts":       "proto-userapis",
	}
	for in, want := range cases {
		if got := protoOwnerRepo(in); got != want {
			t.Errorf("%s → %q, 바람 %q", in, got, want)
		}
	}
}
