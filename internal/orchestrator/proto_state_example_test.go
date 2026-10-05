package orchestrator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitRepoWith(t *testing.T, files map[string]string) string {
	t.Helper()
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
	for name, body := range files {
		os.MkdirAll(filepath.Join(dir, filepath.Dir(name)), 0755)
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	run("add", "-A")
	run("commit", "-q", "-m", "first")
	return dir
}

// 말로 이르는 것과 이웃을 한 번 보여 주는 것은 다르다. 관문이 세 시도 내내
// "int32(3곳)" 이라고 말했는데도 모델은 string 을 냈다.
func TestStateFieldExampleShowsRealLines(t *testing.T) {
	dir := gitRepoWith(t, map[string]string{
		"v1/a.proto": "message A {\n  // 0 이면 정해지지 않음\n  int32 status = 3;  // ceo.v1 Status 와 같은 값\n}\n",
		"v1/b.proto": "message B {\n  int32 state = 5;\n  string name = 1;\n}\n",
	})
	got := protoStateFieldExample(dir)
	for _, want := range []string{"int32 status = 3;", "int32 state = 5;", "v1/a.proto", "string 으로 담지 마라", "enum 은 막지 않는다"} {
		if !strings.Contains(got, want) {
			t.Fatalf("%q 가 없다:\n%s", want, got)
		}
	}
	// 주석까지 보여 줘야 따라 쓸 수 있다.
	if !strings.Contains(got, "ceo.v1 Status 와 같은 값") {
		t.Fatalf("뜻을 적은 주석이 빠졌다:\n%s", got)
	}
	// 상태가 아닌 필드는 안 보여 준다.
	if strings.Contains(got, "string name") {
		t.Fatalf("상태가 아닌 필드가 섞였다:\n%s", got)
	}
}

// 상태 필드가 없는 계약에서는 아무 말도 하지 않는다.
func TestStateFieldExampleQuietWhenNone(t *testing.T) {
	dir := gitRepoWith(t, map[string]string{"v1/a.proto": "message A {\n  string name = 1;\n}\n"})
	if got := protoStateFieldExample(dir); got != "" {
		t.Fatalf("보여 줄 것이 없는데 내놓는다:\n%s", got)
	}
}

// **고치는 중인 트리가 아니라 HEAD 를 본다.** 다시 시킬 때 트리에는 앞 시도가
// 남아 있고, 그것을 관행으로 내밀면 제 실수를 따라 쓰게 된다(#166).
func TestStateFieldExampleIgnoresDirtyTree(t *testing.T) {
	dir := gitRepoWith(t, map[string]string{
		"v1/a.proto": "message A {\n  int32 status = 3;\n  int32 state = 5;\n}\n",
	})
	// 앞 시도가 남긴 엉터리 — 트리에만 있다.
	os.WriteFile(filepath.Join(dir, "v1/a.proto"),
		[]byte("message A {\n  int32 status = 3;\n  int32 state = 5;\n  string pending_status = 9;\n}\n"), 0644)

	got := protoStateFieldExample(dir)
	if strings.Contains(got, "string pending_status") {
		t.Fatalf("앞 시도의 실수를 관행으로 내민다:\n%s", got)
	}
}
