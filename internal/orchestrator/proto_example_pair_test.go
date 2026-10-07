package orchestrator

import (
	"os"
	"strings"
	"testing"
)

// service 파일을 고치는 코더가 본보기를 **하나도 못 받고 있었다.**
//
// `connect.service.proto` 에는 메시지가 없다(`service Internal` 뿐이다).
// 그래서 요청·응답 짝을 못 찾았고, 짝이 없으면 통째로 빈 문자열을 내도록
// 되어 있어 상태 타입 예시(#170)·service 예시(#152)·enum 예시까지 함께
// 사라졌다. 셋은 짝과 아무 상관이 없다.
//
// 실측 W-83289 가 `string status`, W-16789 가 `bool hold_status` 를 낸 자리가
// 정확히 그 파일이다.
func TestServiceFileStillGetsTheExamples(t *testing.T) {
	repo := "/home/cnf/cie-repos/proto-ceowebapis"
	if _, err := os.Stat(repo); err != nil {
		t.Skip("계약 사본이 없다")
	}
	got := protoConventionExample(repo, "ceoweb/v1/connect.service.proto", 2)
	if got == "" {
		t.Fatal("service 파일을 고치는데 본보기를 하나도 안 준다")
	}
	for _, want := range []string{"int32", "service Internal"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q 가 없다:\n%s", want, clipFor(got, 1200))
		}
	}
	// 이웃 파일에서 짝을 가져왔는지.
	if !strings.Contains(got, "요청·응답을 쓰는 꼴") {
		t.Errorf("이웃에서 짝을 못 가져왔다:\n%s", clipFor(got, 800))
	}
	t.Logf("\n%s", clipFor(got, 1400))
}

// 메시지가 있는 파일은 전과 같이 제 파일의 짝을 쓴다.
func TestFileWithPairsUsesItsOwn(t *testing.T) {
	repo := "/home/cnf/cie-repos/proto-ceowebapis"
	if _, err := os.Stat(repo); err != nil {
		t.Skip("계약 사본이 없다")
	}
	got := protoConventionExample(repo, "ceoweb/v1/connect.communication.proto", 2)
	if !strings.Contains(got, "요청·응답을 쓰는 꼴") {
		t.Fatal("짝이 있는 파일인데 짝을 안 보여 준다")
	}
}

func clipFor(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "\n…(줄임)"
}
