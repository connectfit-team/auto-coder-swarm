package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 관문이 도는 자리에서 작업 트리에는 **이미 이번 수정이 들어가 있다.**
// 그대로 세면 새 필드가 자기 자신을 습관으로 투표하고 빠져나간다
// (실측 W-65042 가 그렇게 `string status = 15` 로 승인 대기까지 갔다).
func TestNewFieldDoesNotVoteForItself(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "ceoweb/v1"), 0755); err != nil {
		t.Fatal(err)
	}
	// 작업 트리 — 이번에 넣은 `string status = 15` 가 **이미 들어 있다**.
	tree := `message A {
  int32 status = 3;
  int32 state = 5;
  int32 work_status = 7;
}

message ReceivedRequest {
  string existing_workplace_id = 14;
  string status = 15;
}
`
	if err := os.WriteFile(filepath.Join(dir, "ceoweb/v1/a.proto"), []byte(tree), 0644); err != nil {
		t.Fatal(err)
	}
	diff := "+++ b/ceoweb/v1/a.proto\n+  string status = 15;\n"

	got := stateFieldTypeOffHabit(dir, diff)
	if len(got) != 1 {
		t.Fatalf("새 필드가 자기 표로 빠져나갔다: %v", got)
	}
	all := got[0].Why + " " + strings.Join(got[0].Evidence, " ")
	if strings.Contains(all, "string(") {
		t.Fatalf("습관에 string 이 남아 있다:\n%s", all)
	}
	if !strings.Contains(all, "int32(3곳)") {
		t.Fatalf("이 계약이 쓰는 것을 잘못 셌다:\n%s", all)
	}
}

// 원래부터 string 을 쓰던 계약이면 여전히 조용해야 한다.
func TestGenuineStringHabitStillPasses(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "v1"), 0755)
	tree := `message A {
  string status = 1;
  string state = 2;
  string work_status = 3;
  string new_status = 4;
}
`
	os.WriteFile(filepath.Join(dir, "v1/a.proto"), []byte(tree), 0644)
	diff := "+++ b/v1/a.proto\n+  string new_status = 4;\n"
	if got := stateFieldTypeOffHabit(dir, diff); len(got) != 0 {
		t.Fatalf("string 을 쓰는 계약인데 물었다: %v", got)
	}
}
