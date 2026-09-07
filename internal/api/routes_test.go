package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// 열쇠가 정해져 있으면 **모든** 길이 막혀야 한다.
//
// 전에는 손잡이마다 검사를 불렀고 아홉 중 다섯이 빠져 있었다. 그 가운데
// POST /api/v1/approve 는 사람의 승인을 대신해 PR 을 연다. 목록을 돌며 재면
// 새 길을 더할 때 빠뜨릴 수 없다.
func TestEveryRouteNeedsKey(t *testing.T) {
	t.Setenv("SWARM_API_KEY", "열쇠")

	h := &SwarmHandler{} // 열쇠에서 막히므로 손잡이는 불리지 않는다
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	for pattern := range h.routes() {
		method, path := split(pattern)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(method, path, nil))
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s: 열쇠 없이 %d — 401 이어야 한다", pattern, w.Code)
		}
	}
}

// 열쇠가 맞으면 막지 않는다.
func TestRequireKeyPasses(t *testing.T) {
	t.Setenv("SWARM_API_KEY", "열쇠")
	h := &SwarmHandler{}

	called := false
	r := httptest.NewRequest("GET", "/api/v1/tasks", nil)
	r.Header.Set("X-API-Key", "열쇠")
	h.requireKey(func(http.ResponseWriter, *http.Request) { called = true })(httptest.NewRecorder(), r)
	if !called {
		t.Error("맞는 열쇠를 막았다")
	}
}

// 브라우저의 예비 요청에는 열쇠가 없다. 그것까지 막으면 본 요청이 오지 않는다.
func TestPreflightNeedsNoKey(t *testing.T) {
	t.Setenv("SWARM_API_KEY", "열쇠")
	h := &SwarmHandler{}
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("OPTIONS", "/api/v1/tasks", nil))
	if w.Code != http.StatusOK {
		t.Errorf("예비 요청을 막았다: %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Error("CORS 머리글이 없다")
	}
}

func split(pattern string) (string, string) {
	for i := 0; i < len(pattern); i++ {
		if pattern[i] == ' ' {
			return pattern[:i], pattern[i+1:]
		}
	}
	return "GET", pattern
}
