package apikey

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAllowedHeaderOrCookie(t *testing.T) {
	t.Setenv("SWARM_API_KEY", "열쇠")

	r := httptest.NewRequest("GET", "/", nil)
	if Allowed(r) {
		t.Error("열쇠 없이 통과했다")
	}

	r = httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-API-Key", "틀린것")
	if Allowed(r) {
		t.Error("틀린 열쇠로 통과했다")
	}

	r = httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-API-Key", "열쇠")
	if !Allowed(r) {
		t.Error("맞는 헤더를 막았다")
	}

	// 브라우저는 헤더를 못 붙인다. SetCookie 가 준 것을 되돌려주면 통해야 한다.
	//
	// 열쇠에 한글이 들어 있는 것은 일부러다. 열쇠를 쿠키에 그대로 담았더니
	// net/http 가 ASCII 아닌 바이트를 조용히 버려서, 맞는 열쇠로도 영영
	// 못 들어갔다.
	rec := httptest.NewRecorder()
	SetCookie(rec, "열쇠")
	r = httptest.NewRequest("GET", "/", nil)
	for _, c := range rec.Result().Cookies() {
		r.AddCookie(c)
	}
	if !Allowed(r) {
		t.Error("SetCookie 가 준 쿠키를 막았다")
	}

	r = httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: CookieName, Value: "aaaa"})
	if Allowed(r) {
		t.Error("틀린 쿠키로 통과했다")
	}

	// 열쇠가 바뀌면 예전 쿠키는 못 쓴다.
	t.Setenv("SWARM_API_KEY", "새열쇠")
	r = httptest.NewRequest("GET", "/", nil)
	for _, c := range rec.Result().Cookies() {
		r.AddCookie(c)
	}
	if Allowed(r) {
		t.Error("열쇠를 바꿨는데 예전 쿠키가 통했다")
	}
}

// 열쇠를 정하지 않으면 모두 통과다. 일부러 그렇게 두었다 — 그 대신
// Resolve 가 그 사실을 적는다.
func TestAllowedOpenWhenUnset(t *testing.T) {
	t.Setenv("SWARM_API_KEY", "")
	if !Allowed(httptest.NewRequest("GET", "/", nil)) {
		t.Error("열쇠가 없을 때는 통과여야 한다")
	}
}

// 저장된 열쇠가 다시 떠도 살아 있어야 한다. 이것이 안 되면 대시보드에 넣은
// 열쇠가 몇 시간 뒤 조용히 풀린다.
func TestResolveUsesStoredKey(t *testing.T) {
	t.Setenv("SWARM_API_KEY", "")
	if err := Resolve("저장된열쇠", ":8006"); err != nil {
		t.Fatal(err)
	}
	if Configured() != "저장된열쇠" {
		t.Fatalf("저장된 열쇠를 쓰지 않았다: %q", Configured())
	}
}

// env 가 먼저다 — 드롭인으로 준 열쇠를 DB 값이 덮으면 안 된다.
func TestResolveEnvWins(t *testing.T) {
	t.Setenv("SWARM_API_KEY", "환경열쇠")
	if err := Resolve("저장된열쇠", ":8006"); err != nil {
		t.Fatal(err)
	}
	if Configured() != "환경열쇠" {
		t.Fatalf("env 가 밀렸다: %q", Configured())
	}
}

func TestResolveRequireSwitch(t *testing.T) {
	t.Setenv("SWARM_API_KEY", "")
	t.Setenv("SWARM_REQUIRE_API_KEY", "1")
	if err := Resolve("", ":8006"); err == nil {
		t.Error("열쇠가 없는데 오류를 주지 않았다")
	}
}
