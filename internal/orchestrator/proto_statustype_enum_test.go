package orchestrator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// 꾸러미가 붙은 enum 을 막으면 안 된다.
//
// 실측: 계약 자식 25건 중 9건이 이 관문에서 떨어졌고, 그중에는
// `ceoweb.v1.ConnectionStatus status` 와 `ConnectRequestStatus status` 가
// 있었다. 둘 다 enum 이다 — #167 이 「enum 은 막지 않는다」 로 고친 바로 그
// 경우인데, 이름에 꾸러미가 붙어 글자가 안 맞아 막혔다.
func TestQualifiedEnumIsNotBlocked(t *testing.T) {
	repo := protoRepoForTest(t, `syntax = "proto3";
package ceoweb.v1;

enum ConnectionStatus {
  CONNECTION_STATUS_UNSPECIFIED = 0;
  CONNECTION_STATUS_PENDING = 1;
}

message A {
  int32 status = 1;
}
message B {
  int32 work_state = 2;
}
message C {
  int32 pay_status = 3;
}
`)
	for _, typ := range []string{"ceoweb.v1.ConnectionStatus", "ConnectionStatus", ".ceoweb.v1.ConnectionStatus"} {
		diff := "+++ b/ceoweb/v1/x.proto\n+  " + typ + " pending_status = 15;\n"
		if bad := stateFieldTypeOffHabit(repo, diff); len(bad) > 0 {
			t.Errorf("enum 을 막았다 (%s): %s", typ, bad[0].Why)
		}
	}
}

// enum 이 아닌 것은 그대로 막아야 한다 — 이 관문이 있는 까닭이다.
func TestStringStatusIsStillBlocked(t *testing.T) {
	repo := protoRepoForTest(t, `syntax = "proto3";
package ceoweb.v1;

message A {
  int32 status = 1;
}
message B {
  int32 work_state = 2;
}
message C {
  int32 pay_status = 3;
}
`)
	diff := "+++ b/ceoweb/v1/x.proto\n+  string pending_status = 15;\n"
	bad := stateFieldTypeOffHabit(repo, diff)
	if len(bad) == 0 {
		t.Fatal("string 상태 필드를 안 막았다")
	}
	if !strings.Contains(bad[0].Why, "string") {
		t.Fatalf("무엇이 문제인지 안 밝혔다: %s", bad[0].Why)
	}
	// 지어낸 꾸러미 이름도 막아야 한다 — 그런 enum 은 없다.
	made := "+++ b/ceoweb/v1/x.proto\n+  ceoweb.v1.NoSuchStatus pending_status = 15;\n"
	if len(stateFieldTypeOffHabit(repo, made)) == 0 {
		t.Fatal("없는 타입을 통과시켰다")
	}
}

func TestBaseTypeNameStripsPackage(t *testing.T) {
	for in, want := range map[string]string{
		"ceoweb.v1.ConnectionStatus":  "ConnectionStatus",
		".ceoweb.v1.ConnectionStatus": "ConnectionStatus",
		"ConnectionStatus":            "ConnectionStatus",
		"int32":                       "int32",
		"":                            "",
	} {
		if got := baseTypeName(in); got != want {
			t.Errorf("%q → %q (바라는 것 %q)", in, got, want)
		}
	}
}

// 관문은 살아 있는 저장소를 읽는다 — 작은 것을 하나 만들어 준다.
func protoRepoForTest(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "ceoweb", "v1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ceoweb", "v1", "x.proto"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q"}, {"add", "."},
		{"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "-m", "t"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	return dir
}
