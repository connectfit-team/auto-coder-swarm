package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 같은 개념을 타입마다 다르게 적는 저장소가 있다. 표기를 보여 주지 않으면
// 모델이 하나를 고치고 다른 하나를 깨뜨린다(W-51535).
//
//	StaffSummary.workPlaceId     (대문자 P)
//	ConnectableStaff.workplaceId (소문자 p)
func TestTypeFieldSheet(t *testing.T) {
	dir := t.TempDir()
	w := func(p, s string) {
		full := filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	w("src/lib/server/data/staff.ts", `export interface StaffSummary {
    name: string;
    workPlaceId: string;
    isEmployed?: boolean;
}
export function listStaff() {}
`)
	w("src/lib/server/data/connectmore.ts", `export interface ConnectableStaff {
    staffName: string;
    workplaceId: string;
}
export function listConnectableStaff() {}
`)
	// 파일은 타입 이름을 직접 쓰지 않는다 — 함수로만 부른다.
	w("src/routes/x/+page.server.ts", `import { listStaff } from '$lib/server/data/staff';
import { listConnectableStaff } from '$lib/server/data/connectmore';
export const load = async () => ({ a: listStaff(), b: listConnectableStaff() });
`)

	got := AvailableNames(dir, "src/routes/x/+page.server.ts")
	for _, want := range []string{
		"StaffSummary → ", "workPlaceId",
		"ConnectableStaff → ", "workplaceId",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("%q 가 쪽지에 없다:\n%s", want, got)
		}
	}
}
