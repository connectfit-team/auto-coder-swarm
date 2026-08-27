package orchestrator

import (
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/connectfit-team/auto-coder-swarm/internal/insightclient"
)

// 스킬 문서는 CIE 가 준다. 여기서 이름만 짧게 쓴다.
type SkillDoc = insightclient.SkillDoc

// 팀의 작업 절차를 코더에게 들려 보낸다.
//
// 절차는 코드에 안 적혀 있다. protogen 은 `make push-*apis` 로만 발행해야 하고,
// 생성물은 손으로 고치면 안 되며, main 에 직접 push 하면 안 된다 — 어느 소스
// 파일에도 없는 사실이다. 저장소를 아무리 읽어도 모델은 이걸 알 수 없다.
//
// CIE 쪽에서 배운 것 두 가지를 그대로 가져왔다:
//
//  1. **문서를 어딘가 두는 것만으로는 안 쓴다.** 시스템이 찾아서 프롬프트에 넣는다.
//  2. **넣어도 안 지킨다.** 그래서 기계로 검사할 수 있는 항목은 검사한다
//     (checkProcedureViolations).

// 컨텍스트가 8192 토큰뿐이다. 코더 프롬프트에는 파일 본문이 통째로 들어가므로
// 절차 블록이 커지면 정작 고쳐야 할 코드가 잘린다.
const (
	skillBudgetCoder  = 1500 // 프롬프트에는 파일 본문도 들어간다
	skillBudgetPerDoc = 500
)

// skillDigest 는 프롬프트에 넣을 절차 압축본이다. 제목과 규칙 줄만 남긴다 —
// 예제 코드와 표까지 넣으면 정작 고쳐야 할 파일이 컨텍스트에서 밀려난다.
func skillDigest(docs []SkillDoc) string {
	return renderSkills(docs, skillBudgetCoder, skillBudgetPerDoc)
}

func renderSkills(docs []SkillDoc, budget, perDoc int) string {
	if len(docs) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("[팀의 작업 절차]\n" +
		"이것이 팀이 실제로 하는 방식이다. 코드에서 추측하지 말고 이대로 하라.\n" +
		"어기면 리뷰에서 되돌아오거나, 소비하는 서비스에서 조용히 깨진다.\n\n")

	for _, d := range docs {
		body := trimRunes(strings.TrimSpace(rulesOnly(d.Content)), perDoc)
		if body == "" {
			continue
		}
		mark := ""
		if d.Always {
			mark = " (항상 적용)"
		}
		next := fmt.Sprintf("### %s%s\n%s\n\n", d.Title, mark, body)
		if b.Len()+len(next) > budget {
			break
		}
		b.WriteString(next)
	}
	return b.String()
}

var frontMatter = regexp.MustCompile(`(?s)\A---\n.*?\n---\n`)

// rulesOnly 는 제목·목록 줄만 남긴다. 제목을 같이 남기는 이유는 목록만 떼어내면
// "쓰지 말 것" 아래 항목이 "쓸 것" 처럼 읽히기 때문이다.
func rulesOnly(doc string) string {
	var out []string
	for _, line := range strings.Split(frontMatter.ReplaceAllString(doc, ""), "\n") {
		t := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(t, "#"):
			out = append(out, t)
		case strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* "):
			out = append(out, t)
		}
	}
	return strings.Join(out, "\n")
}

// trimRunes 는 UTF-8 경계에서 자른다. 바이트로 자르면 한국어 문서의 마지막
// 글자가 깨진다.
func trimRunes(s string, max int) string {
	if len(s) <= max {
		return s
	}
	s = s[:max]
	for len(s) > 0 && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s + " …"
}

// ───────── 기계 검사 ─────────

// 생성물은 사람도 기계도 손으로 고치지 않는다. 고쳐도 다음 생성 때 덮어써져
// **조용히 사라지고**, 그 사이 소비하는 서비스만 깨진다.
var generatedFile = regexp.MustCompile(`\.(pb|pb\.gw)\.go$|_grpc\.pb\.go$|\.g\.dart$|\.freezed\.dart$|\.pb\.dart$`)

// diffFile 은 `+++ b/path` 줄에서 경로를 뽑는다.
var diffFile = regexp.MustCompile(`(?m)^\+\+\+ b/(.+)$`)

// checkProcedureViolations 는 절차를 어긴 변경을 찾는다.
//
// 프롬프트에 절차를 넣어도 모델이 지키지 않는 일이 실제로 있었다. 검사할 수 있는
// 것은 검사한다 — 산문 규칙은 지켜지는지 알 수 없다.
func checkProcedureViolations(diff string) []string {
	var out []string
	for _, m := range diffFile.FindAllStringSubmatch(diff, -1) {
		if p := strings.TrimSpace(m[1]); generatedFile.MatchString(p) {
			out = append(out, fmt.Sprintf(
				"생성물 %s 를 손으로 고쳤다. 생성물은 발행 절차(`make push-*apis` 등)가 만드는 것이고, "+
					"손으로 고치면 다음 생성 때 조용히 덮어써진다. 원본(.proto 등)을 고쳐라.", p))
		}
	}
	return out
}

// isGeneratedPath 는 계획 단계에서 생성물을 대상으로 잡았는지 본다.
// 고치기 전에 막는 쪽이 싸다.
func isGeneratedPath(p string) bool {
	return generatedFile.MatchString(filepath.ToSlash(p))
}

// ───────── 조회 ─────────

// fetchSkills 는 이 작업에서 지켜야 할 절차를 CIE 에서 받아 둔다.
//
// 실패해도 작업을 세우지 않는다 — 절차가 없으면 품질이 떨어질 뿐이지만,
// 여기서 막으면 CIE 가 잠깐 흔들릴 때 코딩 자체가 멈춘다. 대신 로그에 남긴다.
func (t *taskContext) fetchSkills(repo string, exts []string) {
	docs, err := t.orchestrator.insightClient.GetSkills(t.ctx, t.req.UserRequest, repo, exts, 3)
	if err != nil {
		log.Printf("⚠️ [ACS] 작업 절차 조회 실패 (절차 없이 진행): %v", err)
		return
	}
	if len(docs) == 0 {
		return
	}
	t.skills = docs

	// 무엇이 실렸는지 남긴다. CIE 에서는 "주입했다고 생각했는데 엉뚱한 문서가
	// 들어가 있던" 일이 있었다 — 추적이 없으면 그걸 못 가른다.
	var titles []string
	for _, d := range docs {
		titles = append(titles, d.Path)
	}
	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "SKILLS",
		fmt.Sprintf("작업 절차 %d개 확보", len(docs)), strings.Join(titles, "\n"), "")
}

// planExtensions 는 계획이 실제로 건드릴 파일의 확장자를 모은다.
// 요청문에 "go" 라고 안 적혀 있어도 Go 규약이 붙게 하려는 것이다.
func planExtensions(paths []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range paths {
		if e := strings.ToLower(filepath.Ext(p)); e != "" && !seen[e] {
			seen[e] = true
			out = append(out, e)
		}
	}
	return out
}
