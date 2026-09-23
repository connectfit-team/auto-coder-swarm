package agent

import (
	"strings"
	"testing"
)

// 검토자가 계약 파일을 짚었는데 버리면 안 된다. 실측으로 맞는 지적이
// 세 회차 내내 버려졌다.
func TestReviewerPointingAtProtoIsKept(t *testing.T) {
	resp := `FEEDBACK: ceoweb/v1/connect.service.proto, 줄 45-46

The new ConnectService definition is added, but it should be added to the
existing Internal service.`
	diff := `--- a/ceoweb/v1/connect.service.proto
+++ b/ceoweb/v1/connect.service.proto
@@
+service ConnectService {
+}
`
	v := ParseReviewerVerdictWithPlan(resp, diff, nil)
	if !v.Blocking {
		t.Fatalf("맞는 지적을 버렸다: %s", v.Why)
	}
	if len(v.Locations) == 0 || !strings.Contains(strings.Join(v.Locations, ","), ".proto") {
		t.Errorf("계약 파일을 자리로 못 읽었다: %v", v.Locations)
	}
}

// 우리가 고치는 말은 다 알아봐야 한다.
func TestGateLocationKnowsOurLanguages(t *testing.T) {
	for _, f := range []string{
		"a/b.proto", "a/b.svelte", "a/b.js", "a/b.mjs", "a/b.py",
		"a/b.go", "a/b.ts", "a/b.json", "a/b.yaml",
	} {
		if got := gateLocation.FindString("FEEDBACK: " + f + " 를 보라"); got != f {
			t.Errorf("%s 를 못 알아봤다: %q", f, got)
		}
	}
}

// 이번 변경에 없는 파일은 여전히 걸러진다 — 넓게 잡아도 새지 않는다.
func TestUnrelatedFileStillFiltered(t *testing.T) {
	v := ParseReviewerVerdictWithPlan(
		"FEEDBACK: 어딘가의 README.md 가 문제다",
		"--- a/x.proto\n+++ b/x.proto\n@@\n+message A {}\n", nil)
	if v.Blocking {
		t.Errorf("이번 변경에 없는 파일로 막았다: %v", v.Locations)
	}
}
