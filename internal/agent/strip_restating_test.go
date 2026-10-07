package agent

import (
	"strings"
	"testing"
)

// 실측 W-71222 이 쓴 주석 그대로다. 계약 수정은 다 맞았는데 이 주석들 때문에
// 세 시도를 다 쓰고 죽었다.
const w71222 = `service Internal {
  rpc RequestProfileUpdate(A) returns (B) {}

  // 내부 서비스에 rpc UpdateReceivedRequestStatus 추가
  rpc UpdateReceivedRequestStatus(RequestUpdateReceivedRequestStatus) returns (ResponseUpdateReceivedRequestStatus) {}
}

// RequestUpdateReceivedRequestStatus 메시지 정의
message RequestUpdateReceivedRequestStatus {
  string request_id = 1;
}

// ResponseUpdateReceivedRequestStatus 메시지 정의
message ResponseUpdateReceivedRequestStatus {
  bool updated = 1;
}
`

func TestStripsTheRestatingComments(t *testing.T) {
	got, n := StripRestatingComments(w71222)
	if n != 3 {
		t.Fatalf("세 줄을 지워야 한다: %d줄\n%s", n, got)
	}
	for _, gone := range []string{"메시지 정의", "rpc UpdateReceivedRequestStatus 추가"} {
		if strings.Contains(got, gone) {
			t.Fatalf("%q 가 남았다:\n%s", gone, got)
		}
	}
	// 코드는 하나도 안 사라져야 한다.
	for _, keep := range []string{
		"rpc UpdateReceivedRequestStatus(RequestUpdateReceivedRequestStatus)",
		"message RequestUpdateReceivedRequestStatus {",
		"bool updated = 1;",
		"rpc RequestProfileUpdate(A) returns (B) {}",
	} {
		if !strings.Contains(got, keep) {
			t.Fatalf("코드가 사라졌다 — %q:\n%s", keep, got)
		}
	}
}

// 뜻이 있는 주석은 그대로 둔다.
func TestKeepsMeaningfulComments(t *testing.T) {
	src := `// 위가 0 이 아닐 때 그 근무지. 화면이 그 직원으로 보낸다.
message ReceivedRequest {
  string id = 1;
}

// 폼은 새로고침·중복 클릭으로 두 번 들어온다.
message RequestCreateInvites {
  string request_id = 1;
}

// 이름이 틀리면 에러 없이 빈 칸이 렌더된다.
func Render() {}
`
	got, n := StripRestatingComments(src)
	if n != 0 {
		t.Fatalf("멀쩡한 주석을 %d줄 지웠다:\n%s", n, got)
	}
	if got != src {
		t.Fatalf("글이 달라졌다:\n%s", got)
	}
}

// 주석이 여러 줄이면 옮겨 적은 줄만 지운다.
func TestStripsOnlyTheRestatingLine(t *testing.T) {
	src := `// 보류는 수락도 거절도 아닌 셋째 상태다.
// PendingStatus 정의
enum PendingStatus {
  PENDING_STATUS_UNSPECIFIED = 0;
}
`
	got, n := StripRestatingComments(src)
	if n != 1 {
		t.Fatalf("한 줄만 지워야 한다: %d줄\n%s", n, got)
	}
	if !strings.Contains(got, "셋째 상태다") {
		t.Fatalf("뜻이 있는 줄이 사라졌다:\n%s", got)
	}
	if strings.Contains(got, "PendingStatus 정의") {
		t.Fatalf("옮겨 적은 줄이 남았다:\n%s", got)
	}
}

// 지울 것이 없으면 글을 건드리지 않는다.
func TestNoChangeWhenNothingToStrip(t *testing.T) {
	src := "package x\n\nfunc F() {}\n"
	got, n := StripRestatingComments(src)
	if n != 0 || got != src {
		t.Fatalf("건드렸다: %d줄\n%s", n, got)
	}
}
