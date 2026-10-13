package main

import "testing"

// 어느 말로 답하든 맞게 재야 한다. 영어로 물으면 영어로 답하는데(상용
// 에이전트도 그렇다), 한국어 글자만 찾으면 멀쩡한 답을 틀렸다고 센다 —
// 실제로 그렇게 0/2 를 냈다.
func TestScoreExpectAnyIsLanguageRobust(t *testing.T) {
	q := Question{Any: []string{"does not import", "없"}}
	for _, answer := range []string{
		"The binary does not import connect_to_database.",
		"그런 함수는 없다.",
	} {
		if ok, why := score(answer, q); !ok {
			t.Errorf("%q → %s", answer, why)
		}
	}
	if ok, _ := score("확실하지 않다", q); ok {
		t.Error("아무 말이나 맞다고 한다")
	}
}

func TestScoreExpectAndForbid(t *testing.T) {
	q := Question{Expect: []string{"dcgettext"}, Forbid: []string{"지어낸함수"}}
	if ok, _ := score("imports dcgettext and strlen", q); !ok {
		t.Error("있는 것을 못 봤다")
	}
	if ok, _ := score("imports strlen", q); ok {
		t.Error("빠졌는데 맞다고 한다")
	}
	if ok, why := score("dcgettext 와 지어낸함수 가 있다", q); ok {
		t.Errorf("없어야 할 것이 있는데 통과시켰다: %s", why)
	}
}
