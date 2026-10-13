package orchestrator

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// 상태 타입은 보통 enum 이다. 「있나 없나」 를 message 로만 물으면 enum 을
// 못 찾고 없다고 답해, 맞게 고른 답이 버려진다.
func TestDeclaredAtHeadFindsEnums(t *testing.T) {
	repo := t.TempDir()
	write := func(rel, body string) {
		full := filepath.Join(repo, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("proto/ceo/connect.proto", `syntax = "proto3";

message ConnectInfo {
  string id = 1;
}

enum PendingStatus {
  PENDING_STATUS_UNSPECIFIED = 0;
  PENDING_STATUS_HELD = 1;
}
`)
	for _, c := range [][]string{
		{"init", "-q"}, {"add", "-A"},
		{"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "x"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, c...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", c, err, out)
		}
	}

	// 시험이 헛돌지 않게 — message 는 양쪽 다 찾아야 한다.
	if got := declaringFileAtHead(repo, "ConnectInfo"); got == "" {
		t.Fatal("message 를 못 찾았다 — 시험 자체가 안 돈다")
	}
	if got := declaredAtHead(repo, "ConnectInfo"); got == "" {
		t.Error("declaredAtHead 가 message 를 못 찾는다")
	}

	// 여기가 갈리는 곳이다.
	if got := declaringFileAtHead(repo, "PendingStatus"); got != "" {
		t.Errorf("declaringFileAtHead 는 message 만 봐야 한다: %q", got)
	}
	if got := declaredAtHead(repo, "PendingStatus"); got != "proto/ceo/connect.proto" {
		t.Errorf("enum 을 찾아야 한다: %q", got)
	}

	// 정말 없는 것은 없다고 해야 한다.
	if got := declaredAtHead(repo, "NoSuchThing"); got != "" {
		t.Errorf("없는 이름을 찾았다고 한다: %q", got)
	}
}
