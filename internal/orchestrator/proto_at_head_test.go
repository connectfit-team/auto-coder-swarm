package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// **HEAD 에 없으면 없는 것이다.** 작업 트리로 물러서면 이번 시도가 새로 만든
// 파일이 「원래 있던 것」 으로 둔갑한다 — #202 에서 못 박은 것과 정반대다.
func TestProtoAtHeadDoesNotReadTheWorktree(t *testing.T) {
	repo := gitRepoWith(t, map[string]string{
		"proto/old.proto": "syntax = \"proto3\";\n\nmessage Old {}\n",
	})
	// 이번 시도가 새 파일을 만들고, 있던 파일도 고쳤다.
	os.WriteFile(filepath.Join(repo, "proto/new.proto"),
		[]byte("syntax = \"proto3\";\n\nmessage BrandNew {}\n"), 0o644)
	os.WriteFile(filepath.Join(repo, "proto/old.proto"),
		[]byte("syntax = \"proto3\";\n\nmessage Old {}\n\nmessage AlsoNew {}\n"), 0o644)

	if got := protoAtHead(repo, "proto/new.proto"); got != "" {
		t.Errorf("HEAD 에 없는 파일을 읽었다: %q", got)
	}
	if got := protoAtHead(repo, "proto/old.proto"); strings.Contains(got, "AlsoNew") {
		t.Error("있던 파일에서 이번에 더한 것까지 읽었다")
	}

	// 그래서 「원래 있던 이름」 판정이 바로 된다.
	if got := declaredAtHead(repo, "Old"); got != "proto/old.proto" {
		t.Errorf("원래 있던 이름을 못 찾았다: %q", got)
	}
	for _, name := range []string{"BrandNew", "AlsoNew"} {
		if got := declaredAtHead(repo, name); got != "" {
			t.Errorf("이번에 만든 %s 를 원래 있던 것이라고 한다: %q", name, got)
		}
	}
}

// git 저장소가 아니면 디스크로 물러선다 — 그때는 HEAD 라는 것이 없다.
func TestProtoAtHeadFallsBackOutsideGit(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "proto"), 0o755)
	os.WriteFile(filepath.Join(dir, "proto/x.proto"), []byte("message X {}\n"), 0o644)

	if got := protoAtHead(dir, "proto/x.proto"); !strings.Contains(got, "message X") {
		t.Errorf("git 밖에서는 읽어야 한다: %q", got)
	}
}
