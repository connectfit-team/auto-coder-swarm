package orchestrator

import (
	"strings"
	"testing"
)

// W-76095 가 넣은 실제 코드. 승인 대기까지 가면 안 된다.
func TestCheckToolLeak(t *testing.T) {
	diff := `diff --git a/src/business/x.go b/src/business/x.go
+++ b/src/business/x.go
+func (s *S) EnsureServiceIsRunning() error {
+	endpoint := "http://127.0.0.1:8000/v1/chat/completions"
+	return s.checkEndpoint(endpoint)
+}`
	bad := CheckToolLeak(diff)
	if len(bad) == 0 {
		t.Fatal("이 기계의 주소를 제품 코드에 넣었는데 통과시켰다")
	}
	if !strings.Contains(AlignmentNote(bad), "127.0.0.1") {
		t.Errorf("무엇이 문제인지 안 보여준다: %s", AlignmentNote(bad))
	}

	// 멀쩡한 변경은 막으면 안 된다.
	ok := `+++ b/src/business/connect.go
+func (c *Connect) HoldConnect(ctx context.Context, id string) error {
+	return c.repository.CEOWorkConnectUpdateState(ctx, id, domain.CEOWorkConnectStateHeld)
+}`
	if bad := CheckToolLeak(ok); len(bad) > 0 {
		t.Errorf("멀쩡한 변경을 막았다: %s", AlignmentNote(bad))
	}

	// 시험 파일에서 제 서버를 띄우는 것은 정상이다.
	tst := `+++ b/src/business/connect_test.go
+	srv := httptest.NewServer(h) // http://127.0.0.1:0 에 뜬다`
	if bad := CheckToolLeak(tst); len(bad) > 0 {
		t.Errorf("시험 코드를 막았다: %s", AlignmentNote(bad))
	}
}
