package orchestrator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// 넘길 때 **상태를 무엇으로 담는지** 함께 알려 줘야 한다.
//
// 코드를 쓸 때 보여 주는 것만으로는 늦다 — 계획이 이미 「string 으로 담는다」
// 로 서 있으면 코더는 계획을 따른다.
func TestHandoffCarriesStateShape(t *testing.T) {
	dir := t.TempDir()
	run := func(a ...string) {
		c := exec.Command("git", append([]string{"-C", dir}, a...)...)
		c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", a, err, out)
		}
	}
	run("init", "-q")
	os.MkdirAll(filepath.Join(dir, "v1"), 0755)
	os.WriteFile(filepath.Join(dir, "v1/a.proto"), []byte(
		"message A {\n  int32 status = 3;\n  int32 state = 5;\n  int32 work_status = 7;\n}\n"), 0644)
	run("add", "-A")
	run("commit", "-q", "-m", "first")

	got := protoStateFieldExample(dir)
	if got == "" {
		t.Fatal("넘길 쪽지에 상태 꼴이 안 들어간다")
	}
	for _, want := range []string{"int32 status = 3;", "string 으로 담지 마라", "enum 은 막지 않는다"} {
		if !strings.Contains(got, want) {
			t.Fatalf("%q 가 없다:\n%s", want, got)
		}
	}
}
