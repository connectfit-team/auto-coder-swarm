package orchestrator

import (
	"strings"
	"testing"
)

// **계약을 어느 길로 정했든 「어느 메시지에 붙이는지」 는 채워야 한다.**
//
// 여태 이 값은 타입을 묻는 갈래에서만 채워졌다. 계약 고르기가 되게 되자
// (#156·#157·#158) 그 갈래를 안 타게 되었고, 넘길 쪽지에서 메시지 이름과 그
// 파일이 통째로 빠졌다. 자식은 넣을 메시지가 없어 새로 만들고 거기 넣었다 —
// 아무도 안 쓰는 자리다(실측 넷 다 그랬다).
//
// 흐름 전체를 돌리지 않고 **그 호출이 남아 있는지**만 본다. 다음 사람이
// 한 갈래에서 지우면 걸린다.
func TestEveryContractPathFillsStateType(t *testing.T) {
	src := readSource(t, "owner_of_state.go")

	// 계약을 정하고 돌아가는 자리는 resolveTracedOwner 를 부르는 곳들이다.
	calls := strings.Count(src, "return t.resolveTracedOwner(c)")
	// 채우는 길은 둘이다 — 따로 묻거나(fillStateType), 이미 물어서 알거나
	// (`t.stateType = typeName`, 타입을 묻는 갈래).
	covered := strings.Count(src, "t.fillStateType(files, missing)") +
		strings.Count(src, "t.stateType = typeName")
	if calls == 0 {
		t.Fatal("계약을 정하는 자리를 못 찾았다 — 이 시험이 지키려던 것이 사라졌다")
	}
	if covered < calls {
		t.Fatalf("계약을 정하는 자리가 %d곳인데 담을 자리가 서는 곳은 %d곳이다 — "+
			"빠진 갈래로 가면 메시지 이름 없이 넘긴다", calls, covered)
	}
}

// 지어낸 이름은 넘기지 않는다. 자식이 그것을 찾다가 못 찾고 새로 만든다.
func TestStateTypeIsValidatedBeforeHandoff(t *testing.T) {
	src := readSource(t, "owner_of_state.go")
	i := strings.Index(src, "func (t *taskContext) fillStateType")
	if i < 0 {
		t.Fatal("fillStateType 이 없다")
	}
	body := src[i:]
	if j := strings.Index(body, "\n}\n"); j > 0 {
		body = body[:j]
	}
	if !strings.Contains(body, "typeHome(") {
		t.Fatalf("있는 이름인지 확인하지 않는다:\n%s", body)
	}
	if !strings.Contains(body, "STATE_TYPE_UNKNOWN") {
		t.Fatalf("못 정했을 때 그 사실을 안 남긴다:\n%s", body)
	}
}
