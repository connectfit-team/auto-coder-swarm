package gitmgr

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// 미는 자리에 문이 있는지 실제로 밀어 보며 확인한다.
//
// 흐름마다 검사를 두면 새 흐름이 그것을 빠뜨린다 — 값 추가 흐름에만 붙였다가
// 결함 흐름 두 자리가 그대로 열려 있었다. 이 시험은 흐름을 거치지 않고
// 미는 함수를 직접 부른다. 여기서 막히면 어느 흐름에서도 막힌다.
func TestPushBlocksMisalignedChange(t *testing.T) {
	work := newRepo(t)

	// 생성물을 손으로 고친다.
	mustWrite(t, filepath.Join(work, "go", "userv1", "message.pb.go"), "package userv1\n")

	m := NewGitManager()
	_, err := m.PushApprovedChangesOpt(work, "some-repo", "feat/add-x", "값 하나 더한다", PushOptions{})
	if !errors.Is(err, ErrMisaligned) {
		t.Fatalf("생성물을 고쳤는데 막지 않았다: %v", err)
	}

	// 막았으면 커밋도 하지 않았어야 한다 — 헛되게 남기지 않는다.
	out, _ := exec.Command("git", "-C", work, "log", "--oneline").Output()
	if n := len(splitLines(string(out))); n != 1 {
		t.Errorf("커밋이 %d개다 — 처음 것 하나여야 한다", n)
	}
}

// 사람이 지키는 가지로는 바로 밀지 않는다.
func TestPushBlocksProtectedBranch(t *testing.T) {
	work := newRepo(t)
	mustWrite(t, filepath.Join(work, "internal", "domain", "t.go"), "package domain\n")

	m := NewGitManager()
	if _, err := m.PushApprovedChangesOpt(work, "some-repo", "master", "고친다", PushOptions{}); !errors.Is(err, ErrMisaligned) {
		t.Fatalf("master 로 미는 것을 막지 않았다: %v", err)
	}
}

func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	run("init", "-q")
	mustWrite(t, filepath.Join(dir, "README.md"), "# t\n")
	run("add", ".")
	run("-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "-m", "처음")
	run("remote", "add", "origin", "https://example.invalid/x.git")
	return dir
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func splitLines(s string) []string {
	var out []string
	for _, l := range []byte(s) {
		if l == '\n' {
			out = append(out, "")
		}
	}
	return out
}
