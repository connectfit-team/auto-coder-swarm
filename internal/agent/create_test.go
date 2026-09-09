package agent

import (
	"os"
	"path/filepath"
	"testing"
)

// 새 파일 만들기의 관문들. 모델을 부르기 전에 걸리는 것만 본다.
func TestCreateFileGuards(t *testing.T) {
	dir := t.TempDir()
	a := &CoderAgent{}

	// 이미 있는 파일은 새로 만들 것이 아니다 — 덮어쓰면 안 된다.
	exists := filepath.Join(dir, "a.go")
	os.WriteFile(exists, []byte("package p\n"), 0o644)
	if _, err := a.CreateFile(t.Context(), exists, "뭐든", ""); err == nil {
		t.Error("이미 있는 파일을 덮어쓰려 했다")
	}

	// 폴더가 없으면 만들지 않는다 — 저장소에 없던 구조가 생긴다.
	if _, err := a.CreateFile(t.Context(), filepath.Join(dir, "없는폴더", "b.go"), "뭐든", ""); err == nil {
		t.Error("없는 폴더에 파일을 만들려 했다")
	}
	if _, err := os.Stat(filepath.Join(dir, "없는폴더")); err == nil {
		t.Error("폴더를 지어냈다")
	}
}

// 이웃 파일 목록을 보여 줘야 같은 꼴로 쓴다.
func TestNeighbourList(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"connect.go", "work.go", "staff.go"} {
		os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644)
	}
	os.Mkdir(filepath.Join(dir, "mock"), 0o755)
	got := neighbourList(dir)
	for _, n := range []string{"connect.go", "work.go", "staff.go"} {
		if !contains(got, n) {
			t.Errorf("이웃 %s 가 없다: %q", n, got)
		}
	}
	if contains(got, "mock") {
		t.Errorf("폴더를 파일로 셌다: %q", got)
	}
}

func contains(s, sub string) bool { return len(sub) > 0 && len(s) >= len(sub) && indexOf(s, sub) >= 0 }
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
