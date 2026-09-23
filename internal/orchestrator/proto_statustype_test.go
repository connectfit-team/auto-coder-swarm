package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 이 계약의 습관: 상태는 전부 int32 다. `content_type` 처럼 글자가 맞는
// 자리도 함께 두어, 그것이 습관에 섞이지 않는지 본다.
const habitProto = `syntax = "proto3";

message A {
  int32 status = 3;
  int32 state = 5;
  int32 work_status = 7;
  string content_type = 2;
  string name = 4;
}
`

func writeHabit(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "ceoweb/v1"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ceoweb/v1/a.proto"), []byte(habitProto), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// 실측 W-13896 이 넣은 것이다.
func TestStringStatusIsOffHabit(t *testing.T) {
	dir := writeHabit(t)
	diff := "+++ b/ceoweb/v1/a.proto\n+  string pending_status = 15;\n"
	got := stateFieldTypeOffHabit(dir, diff)
	if len(got) != 1 {
		t.Fatalf("string 상태 필드를 안 물었다: %v", got)
	}
	all := got[0].Why + " " + strings.Join(got[0].Evidence, " ")
	if !strings.Contains(all, "pending_status") || !strings.Contains(all, "int32") {
		t.Fatalf("무엇을 써야 하는지 안 적혔다:\n%s", all)
	}
	// content_type 이 습관에 섞였으면 string 이 통과해 버린다.
	if strings.Contains(all, "string(") {
		t.Fatalf("글자가 맞는 자리(content_type)가 상태 습관에 섞였다:\n%s", all)
	}
}

func TestHabitualTypesPass(t *testing.T) {
	dir := writeHabit(t)
	if got := stateFieldTypeOffHabit(dir, "+++ b/ceoweb/v1/a.proto\n+  int32 pending_status = 15;\n"); len(got) != 0 {
		t.Fatalf("int32 를 물었다: %v", got)
	}
}

// 상태 필드가 아니면 보지 않는다.
func TestOtherFieldsIgnored(t *testing.T) {
	dir := writeHabit(t)
	diff := "+++ b/ceoweb/v1/a.proto\n" +
		"+  string workplace_id = 15;\n" +
		"+  string status_message = 16;\n" + // status 로 끝나지 않는다
		"+  string content_type = 17;\n" // type 은 보지 않는다
	if got := stateFieldTypeOffHabit(dir, diff); len(got) != 0 {
		t.Fatalf("상태 필드가 아닌 것을 물었다: %v", got)
	}
}

// 표본이 적으면 그것은 습관이 아니다.
func TestThinHabitStaysQuiet(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "v1"), 0755); err != nil {
		t.Fatal(err)
	}
	// 상태 필드가 둘뿐 — 하한(3) 아래다.
	os.WriteFile(filepath.Join(dir, "v1/a.proto"),
		[]byte("message A {\n  int32 status = 1;\n  int32 state = 2;\n}\n"), 0644)
	diff := "+++ b/v1/a.proto\n+  string pending_status = 3;\n"
	if got := stateFieldTypeOffHabit(dir, diff); len(got) != 0 {
		t.Fatalf("한두 자리를 보고 단정한다: %v", got)
	}
}

func TestNoHabitNoVerdict(t *testing.T) {
	dir := t.TempDir()
	if got := stateFieldTypeOffHabit(dir, "+++ b/a.proto\n+  string pending_status = 1;\n"); len(got) != 0 {
		t.Fatalf("견줄 것이 없는데 단정한다: %v", got)
	}
}
