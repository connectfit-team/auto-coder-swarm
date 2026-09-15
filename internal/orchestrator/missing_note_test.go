package orchestrator

import (
	"strings"
	"testing"
)

// 계약 구멍이 없는데 "뒤쪽 저장소" 라고 하면 사람이 엉뚱한 곳을 본다.
func TestMissingNoteSaysWhereTheFaultIs(t *testing.T) {
	onlyLocal := missingNote(map[string][]string{}, []string{"pending", "BulkRow.status"})
	if strings.Contains(onlyLocal, "임자") {
		t.Errorf("이 저장소의 실수뿐인데 남의 저장소를 가리켰다:\n%s", onlyLocal)
	}
	if !strings.Contains(onlyLocal, "여기서 고칠 일이다") {
		t.Errorf("여기서 고치라고 말하지 않았다:\n%s", onlyLocal)
	}

	onlyContract := missingNote(map[string][]string{"proto-ceowebapis": {"ConnectableStaff.hold"}}, nil)
	if !strings.Contains(onlyContract, "proto-ceowebapis") {
		t.Errorf("임자를 안 적었다:\n%s", onlyContract)
	}
	if strings.Contains(onlyContract, "여기서 고칠 일이다") {
		t.Errorf("계약 구멍뿐인데 여기서 고치라고 했다:\n%s", onlyContract)
	}

	both := missingNote(map[string][]string{"proto-ceowebapis": {"ConnectableStaff.hold"}}, []string{"pending"})
	for _, want := range []string{"proto-ceowebapis", "여기서 고칠 일이다"} {
		if !strings.Contains(both, want) {
			t.Errorf("둘 다 있어야 하는데 %q 가 없다:\n%s", want, both)
		}
	}
}
