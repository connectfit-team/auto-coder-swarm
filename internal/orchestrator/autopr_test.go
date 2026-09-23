package orchestrator

import (
	"strings"
	"testing"
)

// 기본값은 꺼 둔 것이다. 적어 둔 저장소에서만 연다.
func TestAutoPRIsOffByDefault(t *testing.T) {
	t.Setenv("SWARM_AUTO_PR_ALLOW", "")
	if autoPRAllowed("gig_ceo_web") {
		t.Error("설정이 비었는데 스스로 PR 을 연다고 했다")
	}
	t.Setenv("SWARM_AUTO_PR_ALLOW", "gig_ceo_web")
	if !autoPRAllowed("gig_ceo_web") {
		t.Error("적어 둔 저장소인데 안 연다고 했다")
	}
	if autoPRAllowed("cms") {
		t.Error("안 적은 저장소인데 연다고 했다")
	}
	if autoPRAllowed("") {
		t.Error("저장소 이름이 없는데 연다고 했다")
	}
	t.Setenv("SWARM_AUTO_PR_ALLOW", "*")
	if !autoPRAllowed("아무거나") {
		t.Error("* 인데 안 연다고 했다")
	}
	// 계약 저장소는 여기서 허락해도 gitmgr 가 막는다(TestContractReposAreNeverPushable).
}

// 읽는 사람이 무엇인지 바로 알아야 하고, 초안으로 열어야 한다.
func TestAutoPRSaysWhatItIs(t *testing.T) {
	lead := autoPRLead("W-123", "검토를 통과한 수정")
	for _, must := range []string{"자동 코딩 시스템이 스스로 만들었습니다", "반드시 검토한 뒤에 머지하세요", "W-123"} {
		if !strings.Contains(lead, must) {
			t.Errorf("PR 머리글에 %q 가 없다", must)
		}
	}
	src := readSource(t, "autopr.go")
	if !strings.Contains(src, "Draft: true") {
		t.Error("초안이 아니라 바로 열 수 있는 PR 로 연다")
	}
	// 머지하는 코드가 있으면 안 된다.
	for _, forbidden := range []string{"pr merge", "MergePR", "--merge"} {
		if strings.Contains(src, forbidden) {
			t.Errorf("스스로 머지하려 한다: %s", forbidden)
		}
	}
}

// 제목은 요청문 첫 줄이다.
func TestPRTitleFrom(t *testing.T) {
	if got := prTitleFrom("연결보류를 더해라\n그리고 화면도"); got != "연결보류를 더해라" {
		t.Errorf("제목이 틀렸다: %q", got)
	}
	if got := prTitleFrom("  \n "); got == "" {
		t.Error("빈 요청에 제목이 없다")
	}
}
