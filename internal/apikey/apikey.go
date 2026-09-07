// 열쇠 하나를 REST API 와 대시보드가 같은 규칙으로 본다.
//
// 규칙이 두 벌이면 한쪽만 잠긴다. 실제로 그랬다 — /api/v1/* 은 X-API-Key 를
// 보는데 같은 포트의 /task/approve 와 /settings/update 는 아무 검사도 없었다.
// 승인은 PR 을 열고, 설정 저장은 열쇠 자체를 덮어쓴다.
package apikey

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
)

const (
	envName = "SWARM_API_KEY"

	// 브라우저는 헤더를 못 붙인다. 그래서 쿠키도 받는다 — 다만 열쇠를 그대로
	// 담지 않고 지문을 담는다(아래 fingerprint).
	CookieName = "swarm_key"
)

// Configured 는 정해진 열쇠를 준다. 빈 문자열이면 정해지지 않았다.
func Configured() string { return os.Getenv(envName) }

// Allowed 는 이 요청을 받아도 되는지 본다.
//
// 열쇠가 정해지지 않았으면 모두 통과다. 그 상태로 도는 것을 감추지 않으려고
// Resolve 가 시작할 때 크게 적는다.
func Allowed(r *http.Request) bool {
	key := Configured()
	if key == "" {
		return true
	}
	if Match(r.Header.Get("X-API-Key"), key) {
		return true
	}
	if c, err := r.Cookie(CookieName); err == nil && Match(c.Value, fingerprint(key)) {
		return true
	}
	return false
}

// Match 는 열쇠를 상수 시간으로 견준다.
func Match(got, want string) bool {
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

// SetCookie 는 이 브라우저가 다음 요청부터 열쇠의 지문을 들고 오게 한다.
// SameSite=Lax 라 다른 사이트가 띄운 POST 에는 쿠키가 붙지 않는다.
func SetCookie(w http.ResponseWriter, key string) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    fingerprint(key),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   30 * 24 * 60 * 60,
	})
}

// Resolve 는 열쇠를 env → 저장된 설정 순으로 정한다.
//
// 대시보드는 열쇠를 DB 와 os.Setenv 양쪽에 넣는다. 다시 뜨면 env 쪽만 사라지고
// DB 에 남은 것을 아무도 읽지 않아서, 그때부터 모두 통과가 된다. 이 기계는
// 하루 세 번 다시 뜬다.
//
// 열쇠가 없어도 뜬다 — 지금 도는 것을 끊지 않는다. 대신 크게 적는다.
// SWARM_REQUIRE_API_KEY 를 주면 그때는 오류를 돌려준다.
func Resolve(stored, listenAddr string) error {
	if Configured() == "" && stored != "" {
		os.Setenv(envName, stored)
		log.Println("🔑 저장된 SWARM_API_KEY 를 쓴다")
	}
	if Configured() != "" {
		return nil
	}
	log.Printf("⚠️  SWARM_API_KEY 가 없다 — %s 가 열쇠 없이 열려 있다. "+
		"이 API 로 만든 작업은 저장소를 고치고 PR 을 연다", listenAddr)
	if os.Getenv("SWARM_REQUIRE_API_KEY") != "" {
		return fmt.Errorf("SWARM_REQUIRE_API_KEY 가 켜져 있는데 열쇠가 없다")
	}
	return nil
}

// fingerprint 는 쿠키에 담을 값을 만든다.
//
// 열쇠를 그대로 담으면 두 가지가 걸린다. 쿠키 값에는 ASCII 아닌 바이트를 담을
// 수 없어서 net/http 가 조용히 버리고(한글이나 공백이 든 열쇠는 아예 통하지
// 않는다), 열쇠 원문이 브라우저 저장소에 남는다. 열쇠에서 만든 지문을 담으면
// 둘 다 없다 — 열쇠를 바꾸면 예전 쿠키는 그 자리에서 못 쓴다.
func fingerprint(key string) string {
	sum := sha256.Sum256([]byte("acs-dashboard\x00" + key))
	return hex.EncodeToString(sum[:])
}
