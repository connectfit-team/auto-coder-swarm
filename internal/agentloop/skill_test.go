package agentloop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func skillDir(t *testing.T, body string, refs map[string]string) string {
	t.Helper()
	root := t.TempDir()
	d := filepath.Join(root, "reverse-engineer-anything")
	if err := os.MkdirAll(filepath.Join(d, "references"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, b := range refs {
		if err := os.WriteFile(filepath.Join(d, "references", name+".md"), []byte(b), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// 지침은 MCP 로 안 온다 — 서버가 파일로 깐다. 안 읽으면 모델이 차례를 모른다
// (실측: open_binary 전에 inspect_binary_layout 을 불러 실패했다).
func TestLoadSkillsReadsBodyAndRefs(t *testing.T) {
	dir := skillDir(t, "# REA\nUse open_binary first.\n",
		map[string]string{"native-and-artifacts": "네이티브는 이렇게 본다."})

	ss := LoadSkills(dir)
	if len(ss) != 1 {
		t.Fatalf("지침을 못 읽었다: %+v", ss)
	}
	if !strings.Contains(ss[0].Body, "Use open_binary first") {
		t.Errorf("본문이 없다: %q", ss[0].Body)
	}
	if _, ok := ss[0].Refs["native-and-artifacts"]; !ok {
		t.Errorf("참고 문서를 못 찾았다: %+v", ss[0].Refs)
	}

	// 지시문에 실린다. 참고 문서는 **이름만** — 다 실으면 창이 안 남는다.
	g := guideBlock(ss)
	if !strings.Contains(g, "Use open_binary first") {
		t.Error("지침이 지시문에 안 들어간다")
	}
	if !strings.Contains(g, "native-and-artifacts") {
		t.Error("참고 문서 이름을 안 알린다")
	}
	if strings.Contains(g, "네이티브는 이렇게 본다") {
		t.Error("참고 문서 본문까지 실었다 — 필요할 때만 읽어야 한다")
	}

	// 필요할 때 읽는다.
	l := &Loop{Skills: ss}
	got, err := l.readReference("native-and-artifacts")
	if err != nil || !strings.Contains(got, "네이티브는 이렇게 본다") {
		t.Errorf("참고 문서를 못 읽었다: %v %q", err, got)
	}
	if _, err := l.readReference("없는문서"); err == nil {
		t.Error("없는 문서를 읽었다고 한다")
	}
}

// 긴 지침은 줄인다 — 창이 16,384 라 통째로는 못 싣는다.
func TestLongSkillIsClipped(t *testing.T) {
	ss := LoadSkills(skillDir(t, strings.Repeat("아주 긴 지침이다. ", 3000), nil))
	if len(ss) != 1 {
		t.Fatal("못 읽었다")
	}
	if n := len([]rune(ss[0].Body)); n > 12000 {
		t.Errorf("안 줄였다: %d자", n)
	}
	if !strings.Contains(ss[0].Body, "줄였다") {
		t.Error("줄였다고 안 알린다 — 조용히 자르면 안 된다")
	}
}

// 지침이 없어도 돌아야 한다.
func TestNoSkillsIsFine(t *testing.T) {
	if ss := LoadSkills(t.TempDir()); len(ss) != 0 {
		t.Errorf("없는데 읽었다: %+v", ss)
	}
	if guideBlock(nil) != "" {
		t.Error("지침이 없는데 글을 만든다")
	}
}
