package agent

import (
	"strings"
	"testing"
)

// 「원문에 없다」 고만 하면 모델은 찾을 줄을 다시 지어낸다. 실은 할 일이
// 남지 않은 것이라고 말해 줘야 한다.
func TestSaysWhenTheChangeIsAlreadyThere(t *testing.T) {
	src := "message A {\n  string id = 1;\n  string hold = 2;\n}\n"
	// 이미 들어가 있는 것을 또 넣으려 한다
	raw := "<<<<<<< SEARCH\n  string nonexistent = 9;\n  string other = 8;\n=======\n  string id = 1;\n  string hold = 2;\n>>>>>>> REPLACE"
	_, err := applyEditBlocks(src, raw)
	if err == nil {
		t.Fatal("없는 줄을 찾으라고 했는데 통과했다")
	}
	if !strings.Contains(err.Error(), "이미 원문에 있다") {
		t.Errorf("이미 들어가 있다고 말하지 않았다: %v", err)
	}
}

// 한 줄짜리는 어디에나 있어 「이미 있다」 가 거짓이 되기 쉽다.
func TestAlreadyThereIgnoresSingleLines(t *testing.T) {
	src := []string{"  string id = 1;", "  string other = 2;"}
	if alreadyThere(src, "  string id = 1;\n") {
		t.Error("한 줄짜리를 「이미 있다」 로 봤다")
	}
	if !alreadyThere(src, "  string id = 1;\n  string other = 2;\n") {
		t.Error("두 줄이 그대로 있는데 못 봤다")
	}
}

// 문장부호가 달라도 같은 자로 견준다.
func TestAlreadyThereFoldsPunctuation(t *testing.T) {
	src := []string{"// 연결은 ceo 가 주인이다 — worker 는 클라이언트다", "string id = 1;"}
	if !alreadyThere(src, "// 연결은 ceo 가 주인이다 - worker 는 클라이언트다\nstring id = 1;\n") {
		t.Error("문장부호 하나 때문에 「없다」 고 했다")
	}
}
