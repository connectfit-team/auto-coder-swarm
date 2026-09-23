package orchestrator

import (
	"strings"
	"testing"
)

// 실측 W-13896 이 쓴 주석 그대로다.
func TestRestatingCommentsCaught(t *testing.T) {
	diff := strings.Join([]string{
		"+++ b/ceoweb/v1/connect.service.proto",
		"+  // 내부 서비스에 UpdateReceivedRequest RPC를 추가한다",
		"+  rpc UpdateReceivedRequest(RequestUpdateReceivedRequest) returns (ResponseUpdateReceivedRequest) {}",
		"+// RequestUpdateReceivedRequest 메시지 정의",
		"+message RequestUpdateReceivedRequest {",
		"+  string request_id = 1;",
		"+}",
	}, "\n")
	got := restatingComments(diff)
	if len(got) != 2 {
		t.Fatalf("둘 다 잡아야 한다: %d개\n%v", len(got), got)
	}
	all := strings.Join(got, " ")
	if !strings.Contains(all, "그대로 옮겨 적었다") || !strings.Contains(all, "왜 그렇게 했는지") {
		t.Fatalf("무엇을 고쳐야 하는지 안 적혔다:\n%s", all)
	}
}

// 멀쩡한 주석을 물면 시도만 깎는다.
func TestGoodCommentsSurvive(t *testing.T) {
	cases := []string{
		"+++ b/a.proto\n+  // 위가 0 이 아닐 때 그 근무지. 화면이 그 직원으로 보낸다.\n+  message ReceivedRequest {",
		"+++ b/a.proto\n+// 폼은 새로고침·중복 클릭으로 두 번 들어온다.\n+message RequestCreateInvites {",
		"+++ b/a.go\n+// 이름이 틀리면 에러 없이 빈 칸이 렌더된다.\n+func Render() {}",
		// 이름이 아예 없는 주석은 옮겨 적은 것이 아니다.
		"+++ b/a.proto\n+// 보류는 수락도 거절도 아닌 셋째 상태다.\n+enum HoldStatus {",
	}
	for _, d := range cases {
		if got := restatingComments(d); len(got) != 0 {
			t.Fatalf("멀쩡한 주석을 물었다:\n%s\n→ %v", d, got)
		}
	}
}

// 주석과 선언 사이에 다른 코드가 있으면 그 주석의 것이 아니다.
func TestCommentMustSitOnTheDecl(t *testing.T) {
	diff := "+++ b/a.proto\n+// RequestX 정의\n+string other = 1;\n+message RequestX {"
	if got := restatingComments(diff); len(got) != 0 {
		t.Fatalf("떨어져 있는 주석을 물었다: %v", got)
	}
}

// 고치지 않은 줄은 보지 않는다.
func TestUntouchedCommentsIgnored(t *testing.T) {
	diff := "+++ b/a.proto\n // RequestOld 정의\n message RequestOld {\n+  string added = 2;"
	if got := restatingComments(diff); len(got) != 0 {
		t.Fatalf("건드리지 않은 주석을 물었다: %v", got)
	}
}

func TestTrimParticle(t *testing.T) {
	for in, want := range map[string]string{
		"서비스에": "서비스", "RPC를": "RPC", "메시지의": "메시지", "내부": "내부", "를": "를",
	} {
		if got := trimParticle(in); got != want {
			t.Fatalf("%q → %q (기대 %q)", in, got, want)
		}
	}
}
