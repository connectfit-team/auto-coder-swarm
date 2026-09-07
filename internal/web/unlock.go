package web

import (
	"fmt"
	"html"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/connectfit-team/auto-coder-swarm/internal/apikey"
)

// 대시보드에는 로그인이 없다. 브라우저는 X-API-Key 헤더를 못 붙이므로,
// 열쇠를 한 번 받아 쿠키로 들고 다니게 한다.
//
// 이 문이 없으면 API 를 잠가도 같은 포트의 /task/approve 와 /settings/update 가
// 열린 채로 남는다 — 승인은 PR 을 열고, 설정 저장은 열쇠를 덮어쓴다.

// gate 는 열쇠가 정해져 있으면 잠근다. 정해지지 않았으면 그대로 통과다.
func (h *DashboardHandler) gate(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if apikey.Allowed(r) {
			next(w, r)
			return
		}
		if r.Method == http.MethodGet {
			http.Redirect(w, r, "/unlock?next="+html.EscapeString(r.URL.RequestURI()), http.StatusSeeOther)
			return
		}
		http.Error(w, "열쇠가 필요하다", http.StatusUnauthorized)
	}
}

func (h *DashboardHandler) HandleUnlock(w http.ResponseWriter, r *http.Request) {
	if apikey.Configured() == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	renderUnlock(w, safeNext(r.URL.Query().Get("next")), "")
}

func (h *DashboardHandler) HandleUnlockSubmit(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	key := apikey.Configured()
	next := safeNext(r.FormValue("next"))
	if key == "" {
		http.Redirect(w, r, next, http.StatusSeeOther)
		return
	}
	if !apikey.Match(r.FormValue("key"), key) {
		// 사내망이라도 열쇠를 무한히 찍어 볼 수 있다. 한 번 틀리면 늦게 답한다.
		log.Printf("[Dashboard] 열쇠가 틀렸다 (from %s)", r.RemoteAddr)
		time.Sleep(time.Second)
		renderUnlock(w, next, "열쇠가 다르다")
		return
	}
	apikey.SetCookie(w, key)
	http.Redirect(w, r, next, http.StatusSeeOther)
}

// safeNext 는 돌아갈 곳을 이 서버 안으로 묶는다.
// 밖으로 보낼 수 있으면 열쇠를 남의 화면으로 유인하는 데 쓰인다.
func safeNext(next string) string {
	if next == "" || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
		return "/"
	}
	return next
}

func renderUnlock(w http.ResponseWriter, next, errMsg string) {
	var note string
	if errMsg != "" {
		note = `<p style="color:#c92a2a;margin:8px 0 0">` + html.EscapeString(errMsg) + `</p>`
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, unlockPage, html.EscapeString(next), note)
}

const unlockPage = `<!doctype html>
<html lang="ko"><head><meta charset="utf-8"><title>열쇠</title>
<style>
 body{font-family:system-ui,-apple-system,sans-serif;display:grid;place-items:center;height:100vh;margin:0;background:#f5f6f7}
 form{background:#fff;padding:28px;border-radius:10px;box-shadow:0 1px 4px rgba(0,0,0,.12);min-width:340px}
 h3{margin:0 0 10px}
 p{color:#666;font-size:.9em;line-height:1.5}
 input{width:100%%;padding:10px;border:1px solid #ddd;border-radius:6px;box-sizing:border-box}
 button{margin-top:12px;width:100%%;padding:10px;border:0;border-radius:6px;background:#2f6feb;color:#fff;font-size:1em}
</style></head>
<body><form method="POST" action="/unlock">
 <h3>🔐 열쇠가 필요하다</h3>
 <p>이 화면은 작업을 승인하고 설정을 바꾼다. REST API 와 같은 X-API-Key 값을
 한 번 넣으면 이 브라우저는 계속 쓸 수 있다.</p>
 <input type="password" name="key" autofocus placeholder="열쇠">
 <input type="hidden" name="next" value="%s">
 <button type="submit">들어간다</button>%s
</form></body></html>`
