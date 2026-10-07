package orchestrator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// 비평가·검토자가 반대해도 **맞게 쓴 것은 남는다.**
//
// 실측 W-11906: 1·2회차에 필드와 RPC 를 둘 다 바르게 넣었는데, 비평가가 enum
// 중복 하나를 짚자 작업 트리가 통째로 지워졌다. 3회차는 처음부터 썼고 RPC 를
// 되살리지 못해, 승인 대기까지 가고도 「바꾸는 길」이 없는 반쪽이 됐다.
//
// 소스에 `checkout .` 이 그 자리에 다시 들어오면 이 시험이 문다.
func TestRejectionKeepsTheWorkInTree(t *testing.T) {
	src, err := os.ReadFile(sourceOf(t, "flow_review.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, mark := range []string{`"CRITIC REJECTION: "`, `"REVIEWER REJECTION: "`} {
		i := strings.Index(string(src), mark)
		if i < 0 {
			t.Fatalf("%s 를 못 찾았다 — 시험이 엉뚱한 자리를 본다", mark)
		}
		// 그 자리에서 다음 return 까지 사이에 되돌리기가 있으면 안 된다.
		rest := string(src)[i:]
		if end := strings.Index(rest, "return false, RunResult{}, nil"); end > 0 {
			rest = rest[:end]
		}
		if strings.Contains(rest, `"checkout", "."`) {
			t.Errorf("%s 뒤에서 작업 트리를 지운다 — 맞게 쓴 것까지 사라진다", mark)
		}
		if !strings.Contains(rest, "위에 적힌 자리만 고쳐라") {
			t.Errorf("%s 에 「그 자리만 고쳐라」 가 없다 — 모델이 처음부터 다시 쓴다", mark)
		}
	}
}

// 통째로 지우는 변경과 공백만 바뀐 변경은 **그대로 되돌려야** 한다.
// 위 시험이 지나치게 넓어져 이것까지 풀어 버리면 안 된다.
func TestHarmfulDiffsAreStillReverted(t *testing.T) {
	src, err := os.ReadFile(sourceOf(t, "flow_review.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, mark := range []string{"DESTRUCTIVE_DIFF", "COSMETIC_DIFF"} {
		i := strings.Index(string(src), mark)
		if i < 0 {
			t.Fatalf("%s 를 못 찾았다", mark)
		}
		rest := string(src)[i:]
		if len(rest) > 700 {
			rest = rest[:700]
		}
		if !strings.Contains(rest, `"checkout", "."`) {
			t.Errorf("%s 에서 되돌리기가 사라졌다 — 이건 되돌리는 게 맞다", mark)
		}
	}
}

// sourceOf 는 이 꾸러미의 소스 파일 경로를 준다.
func sourceOf(t *testing.T, name string) string {
	t.Helper()
	out, err := exec.Command("go", "env", "GOMOD").Output()
	if err != nil {
		t.Skip("go env 를 못 읽었다")
	}
	root := filepath.Dir(strings.TrimSpace(string(out)))
	return filepath.Join(root, "internal", "orchestrator", name)
}
