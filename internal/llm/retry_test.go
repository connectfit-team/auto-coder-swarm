package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

func ask(t *testing.T, base string) (string, error) {
	t.Helper()
	m := NewDirectModel("gemma4:31b", base)
	req := &model.LLMRequest{Contents: []*genai.Content{
		{Role: "user", Parts: []*genai.Part{{Text: "안녕"}}},
	}}
	for resp, err := range m.GenerateContent(context.Background(), req, false) {
		if err != nil {
			return "", err
		}
		if resp != nil && resp.Content != nil && len(resp.Content.Parts) > 0 {
			return resp.Content.Parts[0].Text, nil
		}
	}
	return "", nil
}

func okBody(text string) string {
	b, _ := json.Marshal(map[string]any{
		"choices": []map[string]any{{"message": map[string]string{"content": text}}},
	})
	return string(b)
}

// 재부팅 중에 vLLM 이 5xx 를 내면 다시 건다 — 그 시도를 통째로 버리지 않는다.
func TestRetriesServerErrors(t *testing.T) {
	t.Setenv("LLM_MAX_RETRIES", "2")
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&n, 1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte(okBody("왔다")))
	}))
	defer srv.Close()

	got, err := ask(t, srv.URL)
	if err != nil {
		t.Fatalf("다시 걸지 않았다: %v", err)
	}
	if got != "왔다" || atomic.LoadInt32(&n) != 2 {
		t.Errorf("답=%q 호출=%d", got, n)
	}
}

// **끝난 오류는 다시 걸지 않는다.** 400 은 대개 창 길이 문제라 그대로 다시
// 보내면 또 400 이고, 기다린 만큼만 버린다.
func TestDoesNotRetryTerminalErrors(t *testing.T) {
	t.Setenv("LLM_MAX_RETRIES", "3")
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&n, 1)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"detail":"maximum context length"}`))
	}))
	defer srv.Close()

	if _, err := ask(t, srv.URL); err == nil {
		t.Fatal("400 인데 통과시켰다")
	}
	if atomic.LoadInt32(&n) != 1 {
		t.Errorf("400 을 %d번 걸었다 — 한 번이어야 한다", n)
	}
}

// 다 써도 안 되면 까닭을 들고 끝낸다.
func TestGivesUpWithTheReason(t *testing.T) {
	t.Setenv("LLM_MAX_RETRIES", "0")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("터졌다"))
	}))
	defer srv.Close()

	_, err := ask(t, srv.URL)
	if err == nil {
		t.Fatal("오류를 안 냈다")
	}
	if !strings.Contains(err.Error(), "터졌다") {
		t.Errorf("까닭이 없다: %v", err)
	}
}

func TestWorthRetrying(t *testing.T) {
	cases := []struct {
		status int
		err    error
		want   bool
		what   string
	}{
		{0, context.Canceled, true, "전송 실패"},
		{503, nil, true, "서버가 뜨는 중"},
		{429, nil, true, "너무 많이 불렀다"},
		{400, nil, false, "창 길이"},
		{404, nil, false, "주소가 틀렸다"},
		{200, nil, false, "성공"},
	}
	for _, c := range cases {
		if got := worthRetrying(c.status, c.err); got != c.want {
			t.Errorf("%s(%d): %v 여야 하는데 %v", c.what, c.status, c.want, got)
		}
	}
}
