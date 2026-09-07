package api

import (
	"net/http/httptest"
	"testing"
)

func TestCheckAuthComparesKey(t *testing.T) {
	t.Setenv("SWARM_API_KEY", "열쇠")
	h := &SwarmHandler{}
	r := httptest.NewRequest("GET", "/api/v1/tasks", nil)

	if h.checkAuth(r) {
		t.Error("열쇠 없이 통과했다")
	}
	r.Header.Set("X-API-Key", "틀린것")
	if h.checkAuth(r) {
		t.Error("틀린 열쇠로 통과했다")
	}
	r.Header.Set("X-API-Key", "열쇠")
	if !h.checkAuth(r) {
		t.Error("맞는 열쇠를 막았다")
	}
}

// 열쇠를 정하지 않으면 모두 통과다. 일부러 그렇게 두었지만, 그 상태로 도는 것을
// 아무도 모르면 안 된다 — 시작할 때 resolveAPIKey 가 적는다.
func TestCheckAuthOpenWhenKeyUnset(t *testing.T) {
	t.Setenv("SWARM_API_KEY", "")
	h := &SwarmHandler{}
	if !h.checkAuth(httptest.NewRequest("GET", "/api/v1/tasks", nil)) {
		t.Error("열쇠가 없을 때는 통과여야 한다")
	}
}
