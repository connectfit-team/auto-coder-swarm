package orchestrator

import (
	"strings"
	"testing"

	"github.com/connectfit-team/auto-coder-swarm/internal/insightclient"
)

// 생성물을 손으로 고치면 다음 생성 때 조용히 덮어써진다. 프롬프트에 적어 두는
// 것만으로는 안 지켜져서 기계로 본다.
func TestProcedureViolationCatchesGeneratedFiles(t *testing.T) {
	diff := `diff --git a/api/v1/work.pb.go b/api/v1/work.pb.go
--- a/api/v1/work.pb.go
+++ b/api/v1/work.pb.go
@@
+func (x *Work) GetId() string { return x.Id }
`
	v := checkProcedureViolations(diff)
	if len(v) != 1 || !strings.Contains(v[0], "work.pb.go") {
		t.Fatalf("생성물 수정을 못 잡았다: %v", v)
	}
}

func TestProcedureViolationAllowsNormalFiles(t *testing.T) {
	diff := `diff --git a/internal/business/service.go b/internal/business/service.go
--- a/internal/business/service.go
+++ b/internal/business/service.go
@@
+func New() {}
`
	if v := checkProcedureViolations(diff); len(v) != 0 {
		t.Fatalf("멀쩡한 변경을 위반으로 봤다: %v", v)
	}
}

func TestIsGeneratedPath(t *testing.T) {
	gen := []string{
		"api/work.pb.go", "api/work_grpc.pb.go", "gen/route.pb.gw.go",
		"lib/model.g.dart", "lib/model.freezed.dart", "lib/api.pb.dart",
	}
	for _, p := range gen {
		if !isGeneratedPath(p) {
			t.Errorf("생성물인데 아니라고 한다: %s", p)
		}
	}
	ok := []string{
		"internal/http/handler.go", "lib/screen/home.dart",
		// 이름에 pb 가 들어가도 생성물이 아니다.
		"internal/pbcache/store.go", "lib/pb_helper.dart",
	}
	for _, p := range ok {
		if isGeneratedPath(p) {
			t.Errorf("멀쩡한 파일을 생성물이라 한다: %s", p)
		}
	}
}

// 코더 프롬프트에는 파일 본문이 통째로 들어간다. 절차가 예산을 넘겨 버리면
// 정작 고쳐야 할 코드가 컨텍스트에서 밀려난다.
func TestSkillDigestStaysInBudget(t *testing.T) {
	long := strings.Repeat("- 아주 긴 규칙 줄이다.\n", 400)
	docs := []SkillDoc{
		{Title: "주석 규약", Content: "---\n적용: 항상\n---\n# 주석 규약\n" + long, Always: true},
		{Title: "Go 규약", Content: "# Go 규약\n" + long},
		{Title: "proto 발행 절차", Content: "# proto 발행 절차\n" + long},
	}
	got := skillDigest(docs)
	if len(got) > skillBudgetCoder+skillBudgetPerDoc {
		t.Fatalf("예산 %d 를 넘겼다: %d 바이트", skillBudgetCoder, len(got))
	}
	if !strings.Contains(got, "주석 규약") {
		t.Error("항상 적용 문서가 빠졌다")
	}
	if !strings.Contains(got, "(항상 적용)") {
		t.Error("항상 적용 표시가 없다 — 모델이 선택지로 읽는다")
	}
}

// 압축본은 규칙 줄만 남긴다. 예제 코드와 표까지 넣으면 파일 본문이 밀린다.
func TestRulesOnlyKeepsRulesDropsCode(t *testing.T) {
	doc := "---\n적용: .go\n---\n# Go 규약\n## 생성자\n- 인터페이스를 받는다\n" +
		"```go\nfunc New() {}\n```\n| 계층 | 이름 |\n|---|---|\n평범한 문단이다.\n"
	got := rulesOnly(doc)
	for _, want := range []string{"# Go 규약", "## 생성자", "- 인터페이스를 받는다"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q 가 빠졌다:\n%s", want, got)
		}
	}
	for _, no := range []string{"func New()", "|---|", "평범한 문단", "적용: .go"} {
		if strings.Contains(got, no) {
			t.Errorf("%q 가 남았다:\n%s", no, got)
		}
	}
}

func TestPlanExtensions(t *testing.T) {
	got := planExtensions([]string{"a/b.go", "c/d.go", "e/f.dart", "Makefile"})
	if len(got) != 2 || got[0] != ".go" || got[1] != ".dart" {
		t.Fatalf("확장자 수집이 틀렸다: %v", got)
	}
}

// 빈 목록이면 프롬프트에 아무것도 안 붙어야 한다 — 빈 머리말만 붙으면
// 모델이 "절차가 없다" 로 읽는다.
func TestEmptySkillsRenderNothing(t *testing.T) {
	if skillDigest(nil) != "" || skillDigest([]insightclient.SkillDoc{}) != "" {
		t.Fatal("문서가 없는데 블록이 나왔다")
	}
}
