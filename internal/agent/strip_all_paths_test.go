package agent

import (
	"os"
	"strings"
	"testing"
)

// **모든 쓰기 길에서 턴다.**
//
// 한 길만 털면 그 길로 안 간 회차는 여전히 관문에 막혀 시도가 날아간다.
// 고치는 길(ModifyFile)·새로 만드는 길(CreateFile)·치유기(RepairFile) 셋이다.
func TestEveryWritePathStrips(t *testing.T) {
	for _, f := range []string{"coder.go", "create_file.go"} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("%s 를 못 읽었다: %v", f, err)
		}
		src := string(b)
		writes := strings.Count(src, "os.WriteFile(")
		strips := strings.Count(src, "StripRestatingComments(")
		// 시험 파일을 쓰는 길은 제품 코드가 아니라 뺀다.
		if strings.Contains(src, "testPath") {
			writes--
		}
		if strips < writes {
			t.Errorf("%s: 쓰는 곳 %d, 터는 곳 %d — 빠진 길이 있다", f, writes, strips)
		}
	}
}

// 턴 수는 쌓여야 한다. 한 작업에서 파일을 여럿 고치면 그 합을 알려야 한다.
func TestStrippedCountAccumulatesAndResets(t *testing.T) {
	a := &CoderAgent{}
	a.stripped += 2
	a.stripped += 3
	if n := a.StrippedComments(); n != 5 {
		t.Fatalf("쌓이지 않는다: %d", n)
	}
	if n := a.StrippedComments(); n != 0 {
		t.Fatalf("한 번 알린 뒤 안 비운다: %d", n)
	}
}

// **작업마다 따로여야 한다.** 일꾼이 셋이라 동시에 돈다 — 전역이면 로그가
// 엉뚱한 작업에 붙는다.
func TestStripCountIsPerTask(t *testing.T) {
	a, b := &CoderAgent{}, &CoderAgent{}
	a.stripped += 3
	if n := b.StrippedComments(); n != 0 {
		t.Fatalf("다른 작업의 수가 섞인다: %d", n)
	}
	if n := a.StrippedComments(); n != 3 {
		t.Fatalf("제 수를 못 센다: %d", n)
	}
}
