package orchestrator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// 다시 시킬 때 작업 트리에는 앞 시도의 수정이 남아 있다. 그것을 이웃 예시로
// 내밀면 모델이 **제 실수를 관행인 양 본뜬다.**
func TestExamplesComeFromHeadNotDirtyTree(t *testing.T) {
	dir := t.TempDir()
	run := func(a ...string) {
		t.Helper()
		c := exec.Command("git", append([]string{"-C", dir}, a...)...)
		c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", a, err, out)
		}
	}
	run("init", "-q")
	os.MkdirAll(filepath.Join(dir, "v1"), 0755)
	orig := `message RequestDeleteInvite {
  string invite_id = 1;
}

message ResponseDeleteInvite {
  bool deleted = 1;
}
`
	if err := os.WriteFile(filepath.Join(dir, "v1/a.proto"), []byte(orig), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-q", "-m", "first")

	// 앞 시도가 남긴 엉터리 — 트리에만 있다.
	dirty := orig + `
message RequestBroken {
  string totally_wrong = 1;
}

message ResponseBroken {
  string success_message = 1;
}
`
	if err := os.WriteFile(filepath.Join(dir, "v1/a.proto"), []byte(dirty), 0644); err != nil {
		t.Fatal(err)
	}

	got := protoConventionExample(dir, "v1/a.proto", 2)
	if got == "" {
		t.Fatal("예시를 하나도 못 만들었다")
	}
	if strings.Contains(got, "Broken") || strings.Contains(got, "success_message") {
		t.Fatalf("앞 시도의 수정을 관행으로 내민다:\n%s", got)
	}
	if !strings.Contains(got, "bool deleted = 1;") {
		t.Fatalf("원본의 짝이 안 보인다:\n%s", got)
	}
}

// git 이 없거나 HEAD 를 못 읽으면 트리에서라도 읽는다 — 예시가 없는 것보다 낫다.
func TestFallsBackToTreeWithoutGit(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "v1"), 0755)
	os.WriteFile(filepath.Join(dir, "v1/a.proto"),
		[]byte("message RequestX {\n  string id = 1;\n}\n\nmessage ResponseX {\n  bool ok = 1;\n}\n"), 0644)
	if got := protoConventionExample(dir, "v1/a.proto", 2); !strings.Contains(got, "bool ok = 1;") {
		t.Fatalf("git 저장소가 아닌데 아무것도 못 읽었다:\n%s", got)
	}
}
