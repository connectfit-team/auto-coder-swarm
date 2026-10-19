package llm

import (
	"testing"

	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

func turn(role, text string) *genai.Content {
	return &genai.Content{Role: role, Parts: []*genai.Part{{Text: text}}}
}

// **한 턴짜리는 전과 똑같아야 한다.** ACS 의 다른 에이전트들(계획·코딩·비평·
// 검토)은 사용자 메시지 하나만 보낸다 — 그 결과가 바뀌면 안 된다.
func TestSingleTurnIsUnchanged(t *testing.T) {
	t.Setenv("LLM_ROLE_MESSAGES", "1")
	got := BuildMessages(&model.LLMRequest{Contents: []*genai.Content{turn("user", "안녕")}})
	if len(got) != 1 {
		t.Fatalf("메시지 %d개", len(got))
	}
	if got[0]["role"] != "user" || got[0]["content"] != "안녕" {
		t.Errorf("바뀌었다: %+v", got[0])
	}
}

// 여러 턴은 **역할이 붙은 배열**로 간다. 글자로 적은 역할은 약하다 —
// 모델이 제 지난 턴을 제 것으로 못 알아보면 이어 가지 않고 끝내 버린다.
func TestMultiTurnKeepsRoles(t *testing.T) {
	t.Setenv("LLM_ROLE_MESSAGES", "1")
	got := BuildMessages(&model.LLMRequest{Contents: []*genai.Content{
		turn("user", "a.go 를 읽어라"),
		turn("model", `<tool_call>{"name":"read_file"}</tool_call>`),
		turn("user", "<tool_result>package main</tool_result>"),
	}})
	if len(got) != 3 {
		t.Fatalf("메시지 %d개: %+v", len(got), got)
	}
	want := []string{"user", "assistant", "user"}
	for i, w := range want {
		if got[i]["role"] != w {
			t.Errorf("%d번째 역할: %q (바라는 것 %q)", i, got[i]["role"], w)
		}
	}
	// 역할을 글자로 적지 않는다 — 이제 진짜 역할이 있다.
	for _, m := range got {
		if containsAny(m["content"], "[USER]:", "[ASSISTANT]:") {
			t.Errorf("역할을 글자로도 적었다: %q", m["content"])
		}
	}
}

func TestEmptyRequestStillWorks(t *testing.T) {
	t.Setenv("LLM_ROLE_MESSAGES", "1")
	if got := BuildMessages(&model.LLMRequest{}); len(got) != 1 || got[0]["role"] != "user" {
		t.Errorf("빈 요청에서 깨진다: %+v", got)
	}
	if got := BuildMessages(nil); len(got) != 1 {
		t.Errorf("nil 에서 깨진다: %+v", got)
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
	}
	return false
}

// **기본은 꺼져 있다.** 재 보니 나빴다 — 9/12 → 7/12, 평균 68초 → 14초.
// 도구 결과를 user 로 넣는 한, 역할을 진짜로 붙이면 모델이 그것을 「사람이
// 새로 말을 걸었다」 로 읽고 대화하듯 끝내 버린다.
func TestRolesAreOffByDefault(t *testing.T) {
	t.Setenv("LLM_ROLE_MESSAGES", "")
	got := BuildMessages(&model.LLMRequest{Contents: []*genai.Content{
		turn("user", "a"), turn("model", "b"), turn("user", "c"),
	}})
	if len(got) != 1 || got[0]["role"] != "user" {
		t.Errorf("기본이 켜져 있다: %+v", got)
	}
	if !containsAny(got[0]["content"], "[ASSISTANT]:") {
		t.Errorf("전처럼 눌러 보내지 않는다: %q", got[0]["content"])
	}
}
