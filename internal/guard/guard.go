// Package guard 는 밖으로 나가기 전에 마지막으로 보는 문이다.
//
// 검증(gofmt·빌드·파서)은 "코드가 성립하나" 를 본다. 이 문은 "시킨 일인가" 를
// 본다. 문법이 맞고 빌드도 되는데 시키지 않은 파일을 고치는 것이 가장
// 위험하다 — 사람은 PR 제목을 보고 통과시킨다.
//
// **미는 자리 하나에만 둔다.** 흐름마다 검사를 두면 새 흐름이 그것을
// 빠뜨린다 — 실측으로 값 추가 흐름에만 붙였다가 결함 흐름 두 자리가 그대로
// 열려 있었다. 미는 것은 gitmgr 한 곳이므로 거기서 막아야 막힌다.
package guard

import (
	"fmt"
	"regexp"
	"strings"
)

// Violation 은 어긋난 한 가지다.
type Violation struct {
	Why      string   // 사람이 읽는 까닭
	Evidence []string // 파일(:줄)
}

// 손으로 고치면 안 되는 생성물. proto 배포가 다시 만든다.
var generatedPathRe = regexp.MustCompile(
	`\.pb\.go$|\.pb\.dart$|\.pbenum\.dart$|\.pbjson\.dart$|\.pb\.ts$|_pb2\.py$|` +
		`\.g\.dart$|\.freezed\.dart$|/protos?/|/proto_v2/|/generated/|\.gen\.go$`)

// 어떤 흐름도 건드릴 이유가 없는 것들. 값을 더하든 버그를 고치든 같다.
//
// 스키마(migrations·*.sql)도 여기 든다 — 조직 규칙이 DDL 을 사람의 몫으로
// 두고 있고, 자동으로 열린 스키마 변경 PR 은 눌러 보기 전에는 무해해 보인다.
//
// **이름에 든 낱말로 막지 않는다.** 처음에는 secret·deploy 라는 낱말이 든
// 경로를 다 막았는데, 그러면 멀쩡한 코드가 걸린다 —
// gig_mobile 의 secret.pin.manager.dart 는 앱의 PIN 화면이고,
// deployment.go 는 그냥 Go 파일이다. 설정 파일의 **모양**으로 가린다.
var sensitivePathRe = regexp.MustCompile(
	`(^|/)\.env|` +
		`(^|/)secrets?\.(ya?ml|json|env|txt)$|(^|/)credentials?\.(ya?ml|json)$|` +
		`\.pem$|\.key$|(^|/)id_[rd]sa|\.p12$|\.keystore$|\.jks$|` +
		`(^|/)\.github/|(^|/)Dockerfile|(^|/)docker-compose|` +
		`(^|/)deploy(ment)?\.(ya?ml|yml|json|sh)$|` +
		`\.tf$|(^|/)k8s/|(^|/)helm/|(^|/)kustomization\.ya?ml$|` +
		`(^|/)migrations?/|\.sql$`)

// 사람이 지키는 가지. 여기로 바로 밀지 않는다 — PR 로만 간다.
var protectedBranch = map[string]bool{
	"master": true, "main": true, "develop": true, "release": true,
}

// BeforePush 는 어느 흐름에서든 미는 것을 막을 것을 본다.
func BeforePush(diff, branch string) []Violation {
	var out []Violation
	if protectedBranch[strings.TrimSpace(branch)] {
		out = append(out, Violation{
			Why:      fmt.Sprintf("%s 로 바로 밀려고 했다 — 사람이 지키는 가지다. PR 로만 간다", branch),
			Evidence: []string{branch},
		})
	}
	files := ChangedFiles(diff)
	if gen := matching(files, generatedPathRe); len(gen) > 0 {
		out = append(out, Violation{
			Why:      "생성물을 손으로 고쳤다 — 다시 만들어지면 이 변경은 사라지고, 그 사이 소비자만 깨진다",
			Evidence: gen,
		})
	}
	if sens := matching(files, sensitivePathRe); len(sens) > 0 {
		out = append(out, Violation{
			Why:      "설정·비밀·배포 파일을 건드렸다 — 코드를 고치라는 일이 여기까지 오면 안 된다",
			Evidence: sens,
		})
	}
	return out
}

// InsertOnly 는 더하기만 해야 하는 일에서 지운 줄을 찾는다.
//
// 값 하나를 더하는 일에만 쓴다 — 버그를 고치는 일은 지우는 것이 당연하다.
// 띄어쓰기만 바뀐 줄은 뺀다: gofmt 가 정렬을 다시 하면 지운 줄과 더한 줄이
// 짝으로 나온다.
func InsertOnly(diff string) []Violation {
	var addedLines []string
	added := map[string]bool{}
	for _, l := range strings.Split(diff, "\n") {
		if strings.HasPrefix(l, "+") && !strings.HasPrefix(l, "+++") {
			sq := squeeze(l[1:])
			added[sq] = true
			addedLines = append(addedLines, sq)
		}
	}
	var removed []string
	file := ""
	for _, l := range strings.Split(diff, "\n") {
		if strings.HasPrefix(l, "+++ ") {
			file = trimGitPath(strings.TrimPrefix(l, "+++ "))
			continue
		}
		if !strings.HasPrefix(l, "-") || strings.HasPrefix(l, "---") {
			continue
		}
		body := l[1:]
		sq := squeeze(body)
		if strings.TrimSpace(body) == "" || added[sq] || extendedIn(addedLines, sq) {
			continue
		}
		removed = appendOnce(removed, file+": "+strings.TrimSpace(body))
	}
	if len(removed) == 0 {
		return nil
	}
	return []Violation{{
		Why:      "값을 더하는 일인데 있던 줄을 지웠다 — 더하기만 해야 한다",
		Evidence: removed,
	}}
}

// ChangedFiles 는 diff 에서 고친 파일 경로를 뽑는다.
func ChangedFiles(diff string) []string {
	var out []string
	for _, l := range strings.Split(diff, "\n") {
		if !strings.HasPrefix(l, "+++ ") {
			continue
		}
		p := trimGitPath(strings.TrimPrefix(l, "+++ "))
		if p == "" || p == "/dev/null" {
			continue
		}
		out = appendOnce(out, p)
	}
	return out
}

// Note 는 어긋난 것들을 사람이 읽을 글로 만든다.
func Note(bad []Violation) string {
	var b strings.Builder
	for _, v := range bad {
		fmt.Fprintf(&b, "%s\n", v.Why)
		for i, e := range v.Evidence {
			if i >= 5 {
				fmt.Fprintf(&b, "  … 그 밖에 %d개\n", len(v.Evidence)-5)
				break
			}
			fmt.Fprintf(&b, "  %s\n", e)
		}
	}
	return b.String()
}

// FirstLine 은 까닭의 첫 줄이다. 작업 오류 문구에 쓴다.
func FirstLine(note string) string {
	if i := strings.IndexByte(note, '\n'); i >= 0 {
		return note[:i]
	}
	return note
}

func trimGitPath(s string) string {
	return strings.TrimPrefix(strings.TrimSpace(s), "b/")
}

func squeeze(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func matching(files []string, re *regexp.Regexp) []string {
	var out []string
	for _, f := range files {
		if re.MatchString(f) {
			out = append(out, f)
		}
	}
	return out
}

func appendOnce(xs []string, x string) []string {
	for _, v := range xs {
		if v == x {
			return xs
		}
	}
	return append(xs, x)
}

// extendedIn 은 지운 줄이 **한 군데만 늘어난** 채 더한 줄로 다시 나왔는지 본다.
//
// 한 줄로 적힌 열거에 값을 더하면 그 줄을 늘려야 한다:
//
//	enum T { a, b, notYet }  →  enum T { a, b, c, notYet }
//
// 지운 것이 아니라 늘린 것이다. 앞뒤가 그대로면(가운데 한 군데만 끼워졌으면)
// 늘린 것으로 본다. 그 밖의 바뀜은 무언가 사라진 것이므로 막는다.
func extendedIn(addedLines []string, removed string) bool {
	if len(removed) < 4 {
		return false
	}
	for _, a := range addedLines {
		if singleInsertion(removed, a) {
			return true
		}
	}
	return false
}

// singleInsertion 은 b 가 a 에 한 군데만 끼워 넣은 것인지 본다.
func singleInsertion(a, b string) bool {
	if len(b) <= len(a) {
		return false
	}
	p := 0
	for p < len(a) && a[p] == b[p] {
		p++
	}
	s := 0
	for s < len(a)-p && a[len(a)-1-s] == b[len(b)-1-s] {
		s++
	}
	return p+s == len(a)
}
