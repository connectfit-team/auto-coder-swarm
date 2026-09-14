package orchestrator

import (
	"os"
	"testing"
)

// 실제로 넘어왔던 수정 셋으로 잰다. 지어낸 상황이 아니다.
func TestRealDiffs(t *testing.T) {
	cases := []struct {
		path  string
		block bool
		note  string
	}{
		{"testdata/real_diffs/wrapped-existing-code.patch", true, "있던 함수를 try/catch 로 감싼 것뿐"},
		{"testdata/real_diffs/local-const-only.patch", true, "함수 안 지역 const 한 줄뿐"},
		{"testdata/real_diffs/real-new-symbols.patch", false, "export 함수와 타입 필드를 실제로 더했다"},
	}
	for _, c := range cases {
		b, err := os.ReadFile(c.path)
		if err != nil {
			t.Fatalf("%s 를 못 읽었다: %v", c.path, err)
		}
		bad := CheckNewFeatureAddedSomething(string(b))
		if c.block && len(bad) == 0 {
			t.Errorf("%s 를 통과시켰다 (%s) — 센 것 %v", c.path, c.note, addedDeclarations(string(b)))
		}
		if !c.block && len(bad) != 0 {
			t.Errorf("%s 를 막았다 (%s)", c.path, c.note)
		}
	}
}
