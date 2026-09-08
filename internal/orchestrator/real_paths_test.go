package orchestrator

import (
	"os"
	"path/filepath"
	"testing"
)

// F-02 — 전략이 짚은 경로가 그 저장소에 있는지 본다.
func TestRealPaths(t *testing.T) {
	root := t.TempDir()
	must := func(p string) {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, p)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, p), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	must("internal_v2/grpc/connect.go")
	must("internal/models/ceo.work.job.type.go")

	kept, dropped := realPaths(root, "ceo", []string{
		// 저장소 이름이 붙은 것과 안 붙은 것 둘 다 본다
		"ceo/internal_v2/grpc/connect.go",
		"internal/models/ceo.work.job.type.go",
		// 폴더가 있으면 새 파일도 살린다
		"internal_v2/grpc/connect_hold.go",
		// 다른 저장소의 파일 — W-82668 이 이것을 계획까지 들고 갔다
		"worker/src/ceoapi/connect.go",
		// 저장소 밖으로 나가는 것
		"/etc/passwd",
		"../../secret",
		"",
	})

	wantKept := map[string]bool{
		"internal_v2/grpc/connect.go":          true,
		"internal/models/ceo.work.job.type.go": true,
		"internal_v2/grpc/connect_hold.go":     true,
	}
	if len(kept) != len(wantKept) {
		t.Fatalf("살린 경로가 %d개다: %v", len(kept), kept)
	}
	for _, k := range kept {
		if !wantKept[k] {
			t.Errorf("살리면 안 되는 경로: %s", k)
		}
	}
	if len(dropped) != 4 {
		t.Errorf("뺀 경로가 %d개다: %v", len(dropped), dropped)
	}
}
