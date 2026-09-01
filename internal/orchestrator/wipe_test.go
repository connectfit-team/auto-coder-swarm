package orchestrator

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitInit(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "t@t"},
		{"config", "user.name", "t"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	return dir
}

func commitAll(t *testing.T, dir, msg string) {
	t.Helper()
	exec.Command("git", "-C", dir, "add", "-A").Run()
	if out, err := exec.Command("git", "-C", dir, "commit", "-q", "-m", msg).CombinedOutput(); err != nil {
		t.Fatalf("commit: %v %s", err, out)
	}
}

func TestFindWipedFilesCatchesWholeFileDeletion(t *testing.T) {
	dir := gitInit(t)
	// 1,065줄짜리 스키마를 통째로 지우는 일이 실제로 있었다.
	big := strings.Repeat("model tbl_x { id String }\n", 300)
	os.WriteFile(filepath.Join(dir, "schema.prisma"), []byte(big), 0o644)
	commitAll(t, dir, "init")

	os.WriteFile(filepath.Join(dir, "schema.prisma"), []byte("model tbl_x { id String }\n"), 0o644)

	wiped := findWipedFiles(context.Background(), dir)
	if len(wiped) != 1 || !strings.Contains(wiped[0], "schema.prisma") {
		t.Errorf("통째로 지운 파일을 못 잡았다: %v", wiped)
	}
}

func TestFindWipedFilesAllowsNormalEdit(t *testing.T) {
	dir := gitInit(t)
	var b strings.Builder
	for i := 0; i < 300; i++ {
		b.WriteString("const a = 1;\n")
	}
	os.WriteFile(filepath.Join(dir, "a.ts"), []byte(b.String()), 0o644)
	commitAll(t, dir, "init")

	// 몇 줄만 고친다 — 이건 막으면 안 된다.
	edited := strings.Replace(b.String(), "const a = 1;\n", "const a = 2;\n", 3)
	os.WriteFile(filepath.Join(dir, "a.ts"), []byte(edited), 0o644)

	if wiped := findWipedFiles(context.Background(), dir); len(wiped) != 0 {
		t.Errorf("정상 수정을 막았다: %v", wiped)
	}
}
