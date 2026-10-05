package agent

import (
	"strings"
	"testing"
)

// **한 줄은 닮은 정도로 찾지 않는다.**
//
// 실측으로 `await expect(body).not.toContainText('보류');` 가
// `await expect(body).toContainText('연결');` 에 0.80 넘게 닮았다. 한 줄을
// 받으면 **없는 줄을 있다고 하고 엉뚱한 자리를 고친다.**
func TestSingleLineNeverMatchedByLikeness(t *testing.T) {
	src := []string{
		"line one",
		"\t\tawait expect(body).toContainText('연결');",
		"line three",
	}
	want := []string{"\t\tawait expect(body).not.toContainText('보류');"}

	if r := simRatio(strings.TrimSpace(want[0]), strings.TrimSpace(src[1])); r < 0.80 {
		t.Skipf("이 자료로는 0.80 을 안 넘는다(%.2f) — 시험의 전제가 바뀌었다", r)
	}
	if at := contextAwareAt(src, want); len(at) != 0 {
		t.Fatalf("한 줄을 닮은 정도로 맞췄다: %v", at)
	}
	if at := blockAnchorAt(src, want); len(at) != 0 {
		t.Fatalf("한 줄에 앵커가 걸렸다: %v", at)
	}
}

// 같은 코드가 두 곳에 복사돼 있으면 모델은 블록을 두 번 낸다. 첫 블록이 둘 다
// 고치고 나면 두 번째는 찾을 것이 없다 — **그것은 넘겨도 된다.**
// 줄 수로 거르면 이 흔한 경우가 오류가 된다.
func TestAlreadyThereAcceptsOneLongLine(t *testing.T) {
	src := []string{"a {", "lt: new Date(y, m + 1, 1)", "}"}
	if !alreadyThere(src, "lt: new Date(y, m + 1, 1)") {
		t.Fatal("한 줄이어도 길면 이미 있는 것으로 봐야 한다")
	}
	// 짧은 것은 우연히 맞는다 — 보지 않는다.
	if alreadyThere([]string{"}", "x"}, "}") {
		t.Fatal("짧은 한 줄을 이미 있다고 했다")
	}
}

// 진짜로 못 찾은 블록은 **조용히 사라지면 안 된다.**
func TestUnmatchedBlockIsReportedNotSwallowed(t *testing.T) {
	src := "alpha := 1\nbeta := 2\n"
	raw := "<<<<<<< SEARCH\nalpha := 1\n=======\nalpha := 9\n>>>>>>> REPLACE\n\n" +
		"<<<<<<< SEARCH\n이 저장소에 절대 없는 아주 긴 줄입니다 정말로 없습니다\n=======\nx\n>>>>>>> REPLACE"
	_, err := applyEditBlocks(src, raw)
	if err == nil {
		t.Fatal("못 찾은 블록이 조용히 사라졌다")
	}
	if !strings.Contains(err.Error(), "다시 보내지 마라") {
		t.Fatalf("붙은 블록을 다시 보내지 말라는 말이 없다:\n%s", err)
	}
}
