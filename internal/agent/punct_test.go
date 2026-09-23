package agent

import (
	"strings"
	"testing"
)

// 이 저장소의 주석에는 —·「」·… 가 흔한데 모델은 -·"" 로 받아 적는다.
// 글자 하나 때문에 「원문에 없는 줄」 이 되면 안 된다.
func TestEditBlocksFoldPunctuation(t *testing.T) {
	src := "message A {\n  // 연결은 ceo 가 주인이다 — worker 는 클라이언트일 뿐이다.\n  string id = 1;\n}\n"
	// 모델이 em-dash 를 하이픈으로 적었다
	raw := "<<<<<<< SEARCH\n  // 연결은 ceo 가 주인이다 - worker 는 클라이언트일 뿐이다.\n  string id = 1;\n=======\n  // 연결은 ceo 가 주인이다 — worker 는 클라이언트일 뿐이다.\n  string id = 1;\n  string hold = 2;\n>>>>>>> REPLACE"
	out, err := applyEditBlocks(src, raw)
	if err != nil {
		t.Fatalf("문장부호 하나 때문에 막혔다: %v", err)
	}
	if !strings.Contains(out, "string hold = 2;") {
		t.Errorf("바꾸지 못했다:\n%s", out)
	}
	if !strings.Contains(out, "—") {
		t.Errorf("원문의 문장부호를 잃었다:\n%s", out)
	}
}

func TestNormalizePunct(t *testing.T) {
	cases := map[string]string{
		"가 — 나": "가 - 나",
		"‘가’":   "'가'",
		"“가”":   `"가"`,
		"「가」":   `"가"`,
		"가…":    "가...",
		"가 나":   "가 나",
		"그대로":   "그대로",
	}
	for in, want := range cases {
		if got := normalizePunct(in); got != want {
			t.Errorf("%q → %q, 기대 %q", in, got, want)
		}
	}
}
