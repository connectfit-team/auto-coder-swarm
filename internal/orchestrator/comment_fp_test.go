package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 실제 저장소의 주석을 그대로 넣어 오탐이 없는지 본다.
//
// 이 검사는 멀쩡한 주석을 물면 시도만 깎는다. 사람이 쓴 주석 전부를 diff 인
// 양 넣어 보고, 걸리는 것이 있으면 그 자리를 보여 준다.
func TestNoFalsePositivesOnRealSource(t *testing.T) {
	var b strings.Builder
	files := 0
	for _, root := range []string{".", "../agent", "/home/cnf/cie-repos/proto-ceowebapis/ceoweb/v1"} {
		filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
			if err != nil || fi.IsDir() {
				return nil
			}
			ext := filepath.Ext(p)
			if ext != ".go" && ext != ".proto" {
				return nil
			}
			if strings.HasSuffix(p, "_test.go") {
				return nil
			}
			src, err := os.ReadFile(p)
			if err != nil {
				return nil
			}
			files++
			fmt.Fprintf(&b, "+++ b/%s\n", p)
			for _, ln := range strings.Split(string(src), "\n") {
				b.WriteString("+" + ln + "\n")
			}
			return nil
		})
	}
	if files == 0 {
		t.Skip("읽을 파일이 없다")
	}
	got := restatingComments(b.String())
	t.Logf("파일 %d개를 통째로 넣었다", files)
	if len(got) > 0 {
		for _, g := range got {
			t.Errorf("사람이 쓴 주석을 물었다: %s", g)
		}
	}
}
