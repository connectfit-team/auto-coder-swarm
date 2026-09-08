package orchestrator

import (
	"strings"
	"testing"
)

// F-05 — 빌드가 안 되는 PR 은 초안으로 열고 그 사실을 머리에 적어야 한다.
func TestUnbuildableNote(t *testing.T) {
	if unbuildableNote(nil) != "" {
		t.Error("없는 이름이 없으면 아무 말도 붙이지 않아야 한다")
	}

	note := unbuildableNote([]string{
		"internal/validator/setup.go:202:27: undefined: atdv2.BLE",
		"internal/validator/setup.go:259:13: checkInfo.Ble undefined",
	})
	for _, must := range []string{
		"초안",              // 사람이 제목만 보고 머지하지 않게
		"빌드되지 않는다",        // 무엇이 문제인지
		"배포해도 저절로 생기지 않는다", // 낙관하지 않게
		"atdv2.BLE",       // 무엇이 없는지
	} {
		if !strings.Contains(note, must) {
			t.Errorf("PR 머리말에 %q 가 없다:\n%s", must, note)
		}
	}

	// 많으면 잘라 보여 준다. 사람이 읽을 만큼만.
	many := make([]string, unbuildableShown+5)
	for i := range many {
		many[i] = "x.go:1:1: undefined: N"
	}
	if n := strings.Count(unbuildableNote(many), "> - "); n > unbuildableShown+1 {
		t.Errorf("%d줄이나 적는다 — 잘라야 한다", n)
	}
}

// 없는 이름이 있으면 초안으로 열어야 한다. 그 결정이 코드에 남아 있는지 본다.
func TestUnbuildableOpensDraft(t *testing.T) {
	src := readSource(t, "variant_flow.go")
	if !strings.Contains(src, "len(unbuildable) > 0") {
		t.Error("F-05: 없는 이름이 있어도 초안으로 열지 않는다")
	}
	if !strings.Contains(src, "unbuildableNote(unbuildable)") {
		t.Error("F-05: 무엇이 없는지 PR 머리에 적지 않는다")
	}
}
