package orchestrator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// 빌드가 통과해도 타입은 안 볼 수 있다.
//
// gig_ceo_web 의 빌드는 `vite build` 다. 타입을 **지워서** 번들할 뿐 검사하지
// 않는다. 그래서 W-19079 가 없는 RPC 를 부르는 코드를 넣고도 "빌드 통과" 로
// 승인 대기까지 갔다.
//
//	Property 'updateInviteStatus' does not exist on type 'InternalClient<{}>'
//	Property 'pending' does not exist on type '{ ok: true; count: number; }'
//
// 그렇다고 타입 검사를 통과 조건으로 삼을 수는 없다 — 이 저장소는 손대기
// 전부터 418개다. 늘 실패하는 검사는 검사가 아니다.
//
// **손대기 전보다 늘었는가**를 본다. 실측으로 418 → 424 였고, 늘어난 여섯이
// 전부 지어낸 이름이었다.

type typeError struct{ file, msg string }

var (
	// svelte-check: 경로:줄:칸 다음 줄에 "Error: 메시지"
	svelteLoc = regexp.MustCompile(`^(.+?):\d+:\d+\s*$`)
	// tsc: 경로(줄,칸): error TS1234: 메시지
	tscLine = regexp.MustCompile(`^(.+?)\(\d+,\d+\):\s*error\s+TS\d+:\s*(.+)$`)
)

// parseTypeErrors 는 검사 출력에서 (파일, 메시지) 쌍을 뽑는다.
//
// 줄 번호는 버린다 — 위에 한 줄만 넣어도 아래가 전부 밀려서, 손대기 전과
// 같은 오류가 새 오류로 보인다.
func parseTypeErrors(out string) []typeError {
	var es []typeError
	lines := strings.Split(out, "\n")
	for i, ln := range lines {
		if m := tscLine.FindStringSubmatch(strings.TrimSpace(ln)); m != nil {
			es = append(es, typeError{filepath.Base(m[1]), strings.TrimSpace(m[2])})
			continue
		}
		if m := svelteLoc.FindStringSubmatch(strings.TrimRight(ln, " \t\r")); m != nil && i+1 < len(lines) {
			next := strings.TrimSpace(lines[i+1])
			if strings.HasPrefix(next, "Error:") {
				es = append(es, typeError{
					filepath.Base(m[1]),
					strings.TrimSpace(strings.TrimPrefix(next, "Error:")),
				})
			}
		}
	}
	return es
}

// newTypeErrors 는 손대기 전에 없던 것만 준다(같은 것이 여러 개면 개수까지 본다).
func newTypeErrors(before, after []typeError) []typeError {
	have := map[typeError]int{}
	for _, e := range before {
		have[e]++
	}
	var out []typeError
	for _, e := range after {
		if have[e] > 0 {
			have[e]--
			continue
		}
		out = append(out, e)
	}
	return out
}

// typeCheckCommand 는 이 저장소의 타입 검사 명령이다. 없으면 빈 문자열이다.
//
// Go 는 `go build` 가 이미 타입을 본다. Flutter 는 `flutter analyze` 가 그렇다.
// 타입을 지워서 번들하는 것들만 여기서 따로 본다.
func typeCheckCommand(repoPath string) string {
	b, err := os.ReadFile(filepath.Join(repoPath, "package.json"))
	if err != nil {
		return ""
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if json.Unmarshal(b, &pkg) != nil {
		return ""
	}
	for _, name := range []string{"check", "typecheck", "type-check"} {
		if pkg.Scripts[name] != "" {
			return "npm run " + name
		}
	}
	return ""
}

// typeErrorNote 는 늘어난 오류를 사람과 치유기가 읽을 글로 만든다.
func typeErrorNote(es []typeError) string {
	var b strings.Builder
	b.WriteString("손대기 전에 없던 타입 오류가 생겼다. 지어낸 이름이 아닌지 봐라.\n")
	for i, e := range es {
		if i == 8 {
			b.WriteString("  …\n")
			break
		}
		b.WriteString("  " + e.file + ": " + e.msg + "\n")
	}
	return b.String()
}
