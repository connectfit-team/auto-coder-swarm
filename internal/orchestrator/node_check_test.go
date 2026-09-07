package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TypeScript·Svelte 도 파싱한다. tsc 를 그대로 돌리면 없는 모듈까지 오류로 낸다.
func TestNodeParserCatchesSyntaxOnly(t *testing.T) {
	if tsParserScript() == "" {
		t.Skip("파서 없음")
	}
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	// 없는 모듈을 불러도 문법 오류가 아니다.
	okTS := write("ok.ts", "import { X } from \"./nowhere\";\nexport const a: number = 1;\n")
	badTS := write("bad.ts", "export const a = {\n")
	okSvelte := write("ok.svelte", "<script lang=\"ts\">\n  let x: number = 1;\n</script>\n<div>{x}</div>\n")
	badScript := write("bad1.svelte", "<script lang=\"ts\">\n  let x: number = {\n</script>\n<div>hi</div>\n")
	badMarkup := write("bad2.svelte", "<script lang=\"ts\">\n  let x = 1;\n</script>\n<div>\n")

	if msg := nodeParses(okTS); msg != "" {
		t.Errorf("없는 모듈을 문법 오류로 봤다: %s", msg)
	}
	if nodeParses(badTS) == "" {
		t.Error("깨진 ts 를 통과시켰다")
	}
	if msg := nodeParses(okSvelte); msg != "" {
		t.Errorf("멀쩡한 svelte 를 막았다: %s", msg)
	}
	if nodeParses(badScript) == "" {
		t.Error("깨진 svelte 스크립트를 통과시켰다")
	}
	if nodeParses(badMarkup) == "" {
		t.Error("닫히지 않은 태그를 통과시켰다")
	}
}

// 파서가 파일을 못 찾은 것과 문법이 틀린 것은 다르다.
// 못 찾은 것을 통과로 세면 검사가 조용히 사라지고, 문법 오류로 세면
// 멀쩡한 PR 이 엉뚱한 까닭으로 막힌다.
func TestNodeMissingFileIsNotSyntaxError(t *testing.T) {
	if tsParserScript() == "" {
		t.Skip("파서 없음")
	}
	msg := nodeParses(filepath.Join(t.TempDir(), "없는파일.ts"))
	if msg == "" {
		t.Fatal("없는 파일이 통과했다")
	}
	if !strings.Contains(msg, "파서를 돌리지 못했다") {
		t.Fatalf("문법 오류로 잘못 읽었다: %s", msg)
	}
}
