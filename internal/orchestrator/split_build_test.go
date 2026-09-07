package orchestrator

import "testing"

// 없는 이름은 사람이 만들면 되고, 타입이 안 맞는 것은 우리가 틀린 것이다.
func TestSplitBuildErrors(t *testing.T) {
	missing, wrong := SplitBuildErrors([]string{
		"a.go:78:11: proto.Ble undefined (type *X has no field or method Ble)",
		"b.go:1:1: undefined: NewThing",
		"c.go:12:3: cannot use a (variable of type int) as string value",
	})
	if len(missing) != 2 {
		t.Errorf("없는 이름 %d개: %v", len(missing), missing)
	}
	if len(wrong) != 1 {
		t.Errorf("틀린 것 %d개: %v", len(wrong), wrong)
	}
}
