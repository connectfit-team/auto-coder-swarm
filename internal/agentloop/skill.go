package agentloop

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
	"github.com/connectfit-team/auto-coder-swarm/internal/mcpclient"
)

// 지침 본문이 쓸 수 있는 토큰. 창이 16,384 라 통째로는 못 싣는다.
const guideTokens = 1800

// Skill 은 도구를 **어떻게 쓰는지** 적어 둔 글이다.
//
// MCP 로는 도구와 절차만 온다. 「무엇부터 열어라」 같은 차례는 서버가 **파일로**
// 깔아 둔다 — REA 는 `rea setup` 으로 ~/.agents/skills/<이름>/SKILL.md 를 깐다.
// codex·Claude Code 는 그것을 읽는다. 안 읽으면 모델이 차례를 모른 채 더듬는다
// (실측: open_binary 전에 inspect_binary_layout 을 불러 실패했다).
type Skill struct {
	Name string
	Body string            // 줄인 본문
	Refs map[string]string // 이름 -> 파일 경로. 필요할 때만 읽는다.
}

// LoadSkills 는 깔려 있는 지침을 읽는다. 없으면 빈 목록이다.
//
// tools 를 주면 **그 도구를 이름으로 부르는 절만** 골라 싣는다. 안 주면
// 앞에서부터 줄인다.
func LoadSkills(dir string, tools ...mcpclient.Tool) []Skill {
	if dir == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []Skill
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, e.Name(), "SKILL.md"))
		if err != nil {
			continue
		}
		clipped := pickSections(string(body), tools, guideTokens)
		if agent.EstimateTokens(string(body)) > agent.EstimateTokens(clipped) {
			clipped += "\n\n…(지침을 줄였다 — 더 필요하면 read_reference 를 불러라)"
		}
		s := Skill{Name: e.Name(), Body: clipped, Refs: map[string]string{}}
		refs, _ := os.ReadDir(filepath.Join(dir, e.Name(), "references"))
		for _, r := range refs {
			if strings.HasSuffix(r.Name(), ".md") {
				s.Refs[strings.TrimSuffix(r.Name(), ".md")] =
					filepath.Join(dir, e.Name(), "references", r.Name())
			}
		}
		out = append(out, s)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Name < out[b].Name })
	return out
}

// guideBlock 은 지침을 지시문에 실을 글로 만든다.
func guideBlock(ss []Skill) string {
	if len(ss) == 0 {
		return ""
	}
	var b strings.Builder
	for _, s := range ss {
		fmt.Fprintf(&b, "\n\n[도구 쓰는 차례 — %s]\n%s\n", s.Name, s.Body)
		if len(s.Refs) > 0 {
			var names []string
			for n := range s.Refs {
				names = append(names, n)
			}
			sort.Strings(names)
			fmt.Fprintf(&b, "\n더 자세한 것은 read_reference 로 읽어라: %s\n",
				strings.Join(names, ", "))
		}
	}
	return b.String()
}

// readReference 는 필요할 때만 읽는다 — 다 실으면 창이 안 남는다.
func (l *Loop) readReference(name string) (string, error) {
	for _, s := range l.Skills {
		path, ok := s.Refs[name]
		if !ok {
			continue
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("%s 를 읽지 못했다: %w", name, err)
		}
		out, cut := agent.ClipToTokens(string(body), guideTokens)
		if cut {
			out += "\n…(줄였다)"
		}
		return out, nil
	}
	return "", fmt.Errorf("%s 라는 참고 문서는 없다", name)
}
