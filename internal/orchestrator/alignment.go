package orchestrator

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// 사람이 시킨 것과 어긋나는 편집을 밀어 올리기 전에 막는다.
//
// 값 하나를 더하는 일에서 밖으로 나가는 행동은 하나다 — 브랜치를 밀고 PR 을
// 여는 것. 그 앞에서 실제 편집을 요청과 견준다. 어긋나면 **멈추고 다시
// 시도하지 않는다.** 사람이 볼 근거(파일·줄)를 함께 남긴다.
//
// 검증(gofmt·빌드)은 "코드가 성립하나" 를 보고, 이것은 "시킨 일인가" 를 본다.
// 문법이 맞고 빌드도 되는데 시키지 않은 파일을 고치는 것이 가장 위험하다 —
// 사람은 PR 제목을 보고 통과시킨다.

// Misalignment 는 어긋난 한 가지다.
type Misalignment struct {
	Why      string   // 사람이 읽는 까닭
	Evidence []string // 파일(:줄)
}

// AlignmentInput 은 판단에 필요한 것이다.
type AlignmentInput struct {
	Repo    string
	Value   string   // 더하는 값
	Diff    string   // git diff HEAD
	Named   []string // 요청이 지목한 저장소. 비면 지목하지 않았다
	Blocker bool     // proto 처럼 지목 밖이라도 함께 가야 하는 저장소
}

// 손으로 고치면 안 되는 생성물. proto 배포가 다시 만든다.
var generatedPathRe = regexp.MustCompile(
	`\.pb\.go$|\.pb\.dart$|\.pbenum\.dart$|\.pbjson\.dart$|\.pb\.ts$|_pb2\.py$|` +
		`\.g\.dart$|\.freezed\.dart$|/protos?/|/proto_v2/|/generated/|\.gen\.go$`)

// 값 하나를 더하는 일이 건드릴 이유가 없는 것들. 하나라도 닿으면 멈춘다.
var sensitivePathRe = regexp.MustCompile(
	`(^|/)\.env|secret|credential|(^|/)\.github/|Dockerfile|docker-compose|` +
		`(^|/)deploy|\.tf$|(^|/)k8s/|(^|/)helm/|\.pem$|\.key$|/migrations?/`)

// CheckAlignment 는 이 편집이 시킨 일인지 본다. 빈 목록이면 맞는 것이다.
func CheckAlignment(in AlignmentInput) []Misalignment {
	var out []Misalignment

	files := changedFiles(in.Diff)
	if gen := matching(files, generatedPathRe); len(gen) > 0 {
		out = append(out, Misalignment{
			Why:      "생성물을 손으로 고쳤다 — proto 를 배포하면 다시 만들어지므로 이 변경은 사라지고, 그 사이 소비자만 깨진다",
			Evidence: gen,
		})
	}
	if sens := matching(files, sensitivePathRe); len(sens) > 0 {
		out = append(out, Misalignment{
			Why:      "값 하나를 더하는 일이 설정·비밀·배포 파일을 건드렸다",
			Evidence: sens,
		})
	}
	if del := removedLines(in.Diff); len(del) > 0 {
		out = append(out, Misalignment{
			Why:      "값을 더하는 일인데 있던 줄을 지웠다 — 더하기만 해야 한다",
			Evidence: del,
		})
	}
	if len(in.Named) > 0 && !in.Blocker && !hasRepo(in.Named, in.Repo) {
		out = append(out, Misalignment{
			Why: fmt.Sprintf("요청은 %s 만 지목했는데 %s 를 고쳤다",
				strings.Join(in.Named, "·"), in.Repo),
			Evidence: files,
		})
	}
	return out
}

// changedFiles 는 diff 에서 고친 파일 경로를 뽑는다.
func changedFiles(diff string) []string {
	var out []string
	for _, l := range strings.Split(diff, "\n") {
		if !strings.HasPrefix(l, "+++ ") {
			continue
		}
		p := strings.TrimSpace(strings.TrimPrefix(l, "+++ "))
		p = strings.TrimPrefix(p, "b/")
		if p == "" || p == "/dev/null" {
			continue
		}
		out = appendOnceStr(out, p)
	}
	return out
}

// removedLines 는 지운 줄을 준다. 다만 **띄어쓰기만 바뀐 줄은 뺀다** —
// gofmt 가 정렬을 다시 하면 지운 줄과 더한 줄이 짝으로 나온다.
func removedLines(diff string) []string {
	added := map[string]bool{}
	for _, l := range strings.Split(diff, "\n") {
		if strings.HasPrefix(l, "+") && !strings.HasPrefix(l, "+++") {
			added[squeeze(l[1:])] = true
		}
	}
	var out []string
	file := ""
	for _, l := range strings.Split(diff, "\n") {
		if strings.HasPrefix(l, "+++ ") {
			file = strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(l, "+++ ")), "b/")
			continue
		}
		if !strings.HasPrefix(l, "-") || strings.HasPrefix(l, "---") {
			continue
		}
		body := l[1:]
		if strings.TrimSpace(body) == "" || added[squeeze(body)] {
			continue
		}
		out = appendOnceStr(out, file+": "+strings.TrimSpace(body))
	}
	return out
}

// squeeze 는 띄어쓰기를 하나로 줄인다. 정렬만 바뀐 줄을 같은 줄로 보려고.
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

func hasRepo(named []string, repo string) bool {
	for _, n := range named {
		if strings.EqualFold(n, repo) || strings.EqualFold(filepath.Base(n), repo) {
			return true
		}
	}
	return false
}

// AlignmentNote 는 어긋난 것들을 사람이 읽을 글로 만든다.
func AlignmentNote(bad []Misalignment) string {
	var b strings.Builder
	for _, m := range bad {
		fmt.Fprintf(&b, "%s\n", m.Why)
		for i, e := range m.Evidence {
			if i >= 5 {
				fmt.Fprintf(&b, "  … 그 밖에 %d개\n", len(m.Evidence)-5)
				break
			}
			fmt.Fprintf(&b, "  %s\n", e)
		}
	}
	return b.String()
}
