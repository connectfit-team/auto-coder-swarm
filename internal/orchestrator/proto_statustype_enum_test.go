package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// **enum 은 막지 않는다.** 처음 판이 `ConnectRequestStatus status` 를 「습관 밖」
// 이라고 물었다(실측 W-98684). enum 은 int32 보다 나은 답이고, ACCEPTANCE.md 가
// 기준으로 적어 둔 모양도 enum 꼴이다.
func TestEnumTypedStatusPasses(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "v1"), 0755)
	os.WriteFile(filepath.Join(dir, "v1/a.proto"),
		[]byte("message A {\n  int32 status = 1;\n  int32 state = 2;\n  int32 work_status = 3;\n}\n"), 0644)

	// 같은 수정 안에서 enum 을 만들어 쓴다 — ACCEPTANCE.md 의 기준 모양이다.
	diff := "+++ b/v1/a.proto\n" +
		"+enum PendingStatus {\n" +
		"+  PENDING_STATUS_UNSPECIFIED = 0;\n" +
		"+  PENDING_STATUS_PENDING = 1;\n" +
		"+}\n" +
		"+  PendingStatus pending_status = 15;\n"
	if got := stateFieldTypeOffHabit(dir, diff); len(got) != 0 {
		t.Fatalf("enum 을 물었다 — 이상적인 답을 막는다: %v", got)
	}
}

// 계약에 이미 있는 enum 도 마찬가지다.
func TestExistingEnumTypedStatusPasses(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "v1"), 0755)
	os.WriteFile(filepath.Join(dir, "v1/a.proto"), []byte(
		"enum CEOPolicyType {\n  CEO_POLICY_TYPE_UNSPECIFIED = 0;\n}\n\n"+
			"message A {\n  int32 status = 1;\n  int32 state = 2;\n  int32 work_status = 3;\n}\n"), 0644)
	diff := "+++ b/v1/a.proto\n+  CEOPolicyType policy_state = 9;\n"
	if got := stateFieldTypeOffHabit(dir, diff); len(got) != 0 {
		t.Fatalf("이미 있는 enum 을 물었다: %v", got)
	}
}

// string 은 여전히 막는다 — 값이 굳지 않는다.
func TestStringStillBlocked(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "v1"), 0755)
	os.WriteFile(filepath.Join(dir, "v1/a.proto"),
		[]byte("message A {\n  int32 status = 1;\n  int32 state = 2;\n  int32 work_status = 3;\n}\n"), 0644)
	got := stateFieldTypeOffHabit(dir, "+++ b/v1/a.proto\n+  string pending_status = 15;\n")
	if len(got) != 1 {
		t.Fatalf("string 을 안 물었다: %v", got)
	}
	if !strings.Contains(strings.Join(got[0].Evidence, " "), "enum 은 막지 않는다") {
		t.Fatalf("enum 이 길이라는 것을 안 알려준다: %v", got[0].Evidence)
	}
}
