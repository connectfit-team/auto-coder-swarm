package agent

import (
	"strings"
	"testing"
)

// "없다" 와 "가장 비슷한 곳" 만으로는 같은 실수를 되풀이한다. 실측으로 한
// 파일이 두 번 다 **새로 넣을 줄을 SEARCH 에 적어** 실패했다(W-63343).
func TestNotFoundShowsHowTo(t *testing.T) {
	src := "line one\n\t\tawait expect(body).toContainText('연결');\nline three\n"
	block := "<<<<<<< SEARCH\n\t\tawait expect(body).not.toContainText('보류');\n=======\nx\n>>>>>>> REPLACE"
	_, err := applyEditBlocks(src, block)
	if err == nil {
		t.Fatal("없는 줄인데 통과시켰다")
	}
	msg := err.Error()
	for _, want := range []string{"넣으려는", "<<<<<<< SEARCH", "toContainText('연결')"} {
		if !strings.Contains(msg, want) {
			t.Errorf("%q 가 없다:\n%s", want, msg)
		}
	}
	// 줄 번호가 **본보기 안에** 섞이면 그대로 베껴 또 틀린다.
	// (위의 「가장 비슷한 곳」 목록에는 번호가 있는 것이 맞다.)
	i := strings.Index(msg, "<<<<<<< SEARCH")
	if i < 0 {
		t.Fatal("본보기가 없다")
	}
	for _, line := range strings.Split(msg[i:], "\n") {
		t := strings.TrimSpace(line)
		if len(t) > 2 && t[0] >= '0' && t[0] <= '9' && strings.Contains(t[:4], ":") {
			t2 := line
			_ = t2
			panicIfNumbered(line)
		}
	}
}

func TestFirstAnchorLine(t *testing.T) {
	near := "원문에서 가장 비슷한 곳은 104줄이다 (낱말 4/4 겹침):\n104: \t\tawait expect(body).toContainText('연결');\n105: \t\tawait expect(body).toContainText('직원');"
	got := firstAnchorLine(near)
	if !strings.Contains(got, "toContainText('연결')") || strings.Contains(got, "104") {
		t.Errorf("원문 한 줄을 못 뽑았다: %q", got)
	}
}

// panicIfNumbered 는 본보기에 줄 번호가 섞였는지 본다.
func panicIfNumbered(line string) {
	panic("본보기에 줄 번호가 들어갔다: " + line)
}
