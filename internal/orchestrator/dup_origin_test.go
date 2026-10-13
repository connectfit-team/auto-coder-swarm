package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 「두 번 선언한다 — a.proto · b.proto」 만으로는 **무엇을 지울지** 모른다.
// HEAD 에 있던 쪽이 원래 것이고, 이번에 더한 쪽을 지워야 한다.
func TestDuplicateSaysWhichOneIsOriginal(t *testing.T) {
	repo := gitRepoWith(t, map[string]string{
		"proto/a.proto": "syntax = \"proto3\";\npackage p;\n\nmessage Dup {\n  string id = 1;\n}\n",
		"proto/b.proto": "syntax = \"proto3\";\npackage p;\n",
	})
	// 자식이 b.proto 에 같은 이름을 또 만들었다.
	full := filepath.Join(repo, "proto/b.proto")
	body, _ := os.ReadFile(full)
	if err := os.WriteFile(full, append(body, []byte("\nmessage Dup {\n  bool held = 1;\n}\n")...), 0o644); err != nil {
		t.Fatal(err)
	}

	bad := duplicateDecls(repo, []string{"proto/b.proto"})
	if len(bad) == 0 {
		t.Fatal("중복을 못 잡았다")
	}
	note := AlignmentNote(bad)
	if !strings.Contains(note, "원래 있던 것은 proto/a.proto") {
		t.Errorf("원래 있던 자리를 안 짚어 준다:\n%s", note)
	}
	if !strings.Contains(note, "이번에 더한 쪽을 지워라") {
		t.Errorf("무엇을 지울지 안 알려 준다:\n%s", note)
	}
}

// HEAD 에 없으면 둘 다 이번에 만든 것이다 — 그렇게 말해야 한다.
func TestDuplicateBothNew(t *testing.T) {
	repo := gitRepoWith(t, map[string]string{"proto/a.proto": "syntax = \"proto3\";\npackage p;\n"})
	for _, rel := range []string{"proto/a.proto", "proto/b.proto"} {
		full := filepath.Join(repo, rel)
		body, _ := os.ReadFile(full)
		if len(body) == 0 {
			body = []byte("syntax = \"proto3\";\npackage p;\n")
		}
		os.WriteFile(full, append(body, []byte("\nmessage NewDup {\n  bool x = 1;\n}\n")...), 0o644)
	}
	bad := duplicateDecls(repo, []string{"proto/a.proto", "proto/b.proto"})
	if len(bad) == 0 {
		t.Fatal("중복을 못 잡았다")
	}
	if note := AlignmentNote(bad); !strings.Contains(note, "둘 다 이번에 더한 것이다") {
		t.Errorf("둘 다 새 것이라고 말하지 않는다:\n%s", note)
	}
}
