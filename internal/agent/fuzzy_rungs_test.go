package agent

import (
	"strings"
	"testing"
)

// 모델이 진짜 줄바꿈 대신 `\n` 을 적어 보내는 것은 흔한 실패다.
func TestEscapedNewlinesInSearch(t *testing.T) {
	src := "func f() {\n\tif x {\n\t\treturn 1\n\t}\n}\n"
	raw := "<<<<<<< SEARCH\n" +
		`if x {\n\t\treturn 1` + "\n" +
		"=======\n" +
		"if x {\n\t\treturn 2\n" +
		">>>>>>> REPLACE"
	got, err := applyEditBlocks(src, raw)
	if err != nil {
		t.Fatalf("이스케이프를 못 폈다: %v", err)
	}
	if !strings.Contains(got, "return 2") {
		t.Fatalf("안 바뀌었다:\n%s", got)
	}
}

// 첫 줄·마지막 줄은 맞는데 가운데를 조금 다르게 적어 온 경우.
func TestBlockAnchorFindsIt(t *testing.T) {
	srcLines := []string{
		"func handle() {",
		"\t// 연결 요청을 수락한다",
		"\tif req.Status == 1 {",
		"\t\treturn accept(req)",
		"\t}",
		"}",
	}
	want := []string{
		"func handle() {",
		"// 연결 요청을 수락한다고 적혀 있었다",
		"if req.Status == 1 {",
		"return accept(req)",
		"}",
		"}",
	}
	at := blockAnchorAt(srcLines, want)
	if len(at) != 1 || at[0] != 0 {
		t.Fatalf("첫 줄·마지막 줄이 맞는 자리를 못 찾았다: %v", at)
	}
}

// 후보가 여럿이면 더 엄하게 본다 — 문턱이 0.70 으로 오른다.
func TestBlockAnchorTightensWithManyCandidates(t *testing.T) {
	var srcLines []string
	for i := 0; i < 2; i++ {
		srcLines = append(srcLines, "func f() {", "\tsomething entirely different here", "}")
	}
	want := []string{"func f() {", "가운데가 전혀 다르다", "}"}
	if at := blockAnchorAt(srcLines, want); len(at) != 0 {
		t.Fatalf("가운데가 전혀 다른데 맞다고 했다: %v", at)
	}
}

// 줄마다 조금씩 다른 것은 마지막 단이 받는다.
func TestContextAwareFindsNearlyIdentical(t *testing.T) {
	srcLines := []string{
		"const limit = 10",
		"const name = \"연결보류\"",
		"const on = true",
	}
	want := []string{
		"const limit = 10",
		"const name = \"연결보류\"",
		"const on = true",
	}
	if at := contextAwareAt(srcLines, want); len(at) != 1 {
		t.Fatalf("똑같은데 못 찾았다: %v", at)
	}
	// 한 줄이 크게 다르면 안 받는다.
	want[1] = "const totally = \"different\""
	if at := contextAwareAt(srcLines, want); len(at) != 0 {
		t.Fatalf("크게 다른데 받았다: %v", at)
	}
}

func TestSimRatio(t *testing.T) {
	if r := simRatio("abc", "abc"); r != 1 {
		t.Fatalf("같은 글이 1 이 아니다: %v", r)
	}
	if r := simRatio("", ""); r != 1 {
		t.Fatalf("둘 다 비면 1 이어야 한다: %v", r)
	}
	if r := simRatio("abc", ""); r != 0 {
		t.Fatalf("한쪽이 비면 0 이어야 한다: %v", r)
	}
	if r := simRatio("연결 요청을 수락한다", "연결 요청을 거절한다"); r < 0.6 || r >= 1 {
		t.Fatalf("거의 같은 글의 닮은 정도가 이상하다: %v", r)
	}
	if r := simRatio("완전히 다른 글", "abcdefghijk"); r > 0.2 {
		t.Fatalf("전혀 다른 글이 닮았다고 한다: %v", r)
	}
}

// **유사도로 찾은 자리는 딱 하나일 때만 쓴다.**
func TestApproxOnlyWhenUnique(t *testing.T) {
	src := strings.Join([]string{
		"func a() {", "\t조금 다른 가운데 하나", "}",
		"func a() {", "\t조금 다른 가운데 둘", "}",
	}, "\n")
	raw := "<<<<<<< SEARCH\nfunc a() {\n가운데가 살짝 다르다\n}\n=======\nfunc a() {\n바뀐 것\n}\n>>>>>>> REPLACE"
	if _, err := applyEditBlocks(src, raw); err == nil {
		t.Fatal("비슷한 자리가 둘인데 골라서 바꿨다")
	}
}

// 유사도 단이 물면 그 사실이 남아야 한다.
func TestApproxRungIsReported(t *testing.T) {
	LastApproxRung() // 비우고 시작
	srcLines := []string{"func f() {", "\t// 원래 주석이 여기 있었다", "\treturn 1", "}"}
	src := strings.Join(srcLines, "\n")
	raw := "<<<<<<< SEARCH\nfunc f() {\n// 원래 주석이 여기 있었다고 한다\nreturn 1\n}\n=======\n" +
		"func f() {\n// 원래 주석이 여기 있었다\nreturn 2\n}\n>>>>>>> REPLACE"
	if _, err := applyEditBlocks(src, raw); err != nil {
		t.Skipf("이 자료로는 아랫단까지 안 갔다: %v", err)
	}
	if LastApproxRung() == "" {
		t.Fatal("닮은 정도로 찾았는데 아무 말도 안 남겼다")
	}
}
