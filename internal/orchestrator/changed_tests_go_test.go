package orchestrator

import (
	"os"
	"path/filepath"
	"testing"
)

// 계약(.proto)만 고친 폴더에는 돌릴 Go 꾸러미가 없다.
// `go test ./그폴더/...` 는 그때 「matched no packages」 로 실패한다 —
// 돌릴 것이 없는 것과 실패는 다르다.
func TestGoTestableDirsSkipsDirsWithoutGo(t *testing.T) {
	root := t.TempDir()
	mk := func(rel string, body string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("ceoweb/v1/connect.service.proto", "syntax = \"proto3\";\n")
	mk("go/ceo/connect/ceoweb/connect.pb.go", "package ceoweb\n")
	mk("internal/thing/thing.go", "package thing\n")

	got := goTestableDirs(root, []string{"ceoweb/v1", "internal/thing", "없는폴더"})
	if len(got) != 1 || got[0] != "internal/thing" {
		t.Errorf("Go 파일이 없는 폴더를 남겼다: %v", got)
	}
}

// 아래 어딘가에 있으면 센다 — go test 는 ./d/... 로 훑는다.
func TestHasGoFilesLooksDeep(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(deep, "x.go"), []byte("package c\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !hasGoFiles(filepath.Join(root, "a")) {
		t.Error("깊이 있는 Go 파일을 못 봤다")
	}
	if hasGoFiles(filepath.Join(root, "없음")) {
		t.Error("없는 폴더를 있다고 했다")
	}
}
