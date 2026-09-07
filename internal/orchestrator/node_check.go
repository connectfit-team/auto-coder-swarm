package orchestrator

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// TypeScript·Svelte 도 파싱한다.
//
// tsc 를 그대로 돌리면 없는 모듈까지 오류로 낸다 — 의존성을 받아 두지 않았다.
// 파서만 부르는 작은 스크립트를 둔다(tools/tsparse/parse.js →
// scripts/install-checkers.sh 가 ~/tools/tsparse 로 깐다).
// .svelte 는 스크립트와 마크업을 나눠 각각 맞는 파서로 본다 — Svelte 의 파서는
// 스크립트를 JS 로 읽어서 lang="ts" 의 타입 표기를 문법 오류로 낸다.
//
// 종료 코드로 문법 오류와 못 돈 것을 가른다: 1 은 문법 오류, 2 는 파일을 못
// 찾았거나 모듈이 없는 것. 못 돈 것을 통과로 세면 검사가 조용히 사라진다.

var nodeParsedExts = map[string]bool{
	".ts": true, ".js": true, ".mjs": true, ".svelte": true,
}

// tsParserScript 는 파서 스크립트의 경로를 준다. 없으면 빈 문자열이다.
func tsParserScript() string {
	if p := strings.TrimSpace(os.Getenv("TS_PARSER")); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	p := filepath.Join(home, "tools", "tsparse", "parse.js")
	if _, err := os.Stat(p); err != nil {
		return ""
	}
	if _, err := exec.LookPath("node"); err != nil {
		return ""
	}
	return p
}

// nodeParses 는 그 파일이 TypeScript·Svelte 로 읽히는지 본다.
// 빈 문자열이면 통과, 그 밖은 사람이 읽을 까닭이다.
// 파서가 아예 없으면 판단하지 않는다(빈 문자열) — 그것은 따로 알린다.
func nodeParses(path string) string {
	script := tsParserScript()
	if script == "" {
		return ""
	}
	cmd := exec.Command("node", script, path)
	cmd.Dir = filepath.Dir(script)
	b, err := cmd.CombinedOutput()
	if err == nil {
		return ""
	}
	msg := firstLineOf(strings.TrimSpace(string(b)))
	if exitCode(err) != 1 {
		// 1 만 문법 오류다. 그 밖은 파서가 돌지 못한 것이다 — 파일을 못
		// 찾았거나 모듈이 없다. 우리 쪽 문제이므로 조용히 넘기면 안 된다.
		return "파서를 돌리지 못했다: " + msg
	}
	if msg == "" {
		return "파서가 까닭 없이 실패했다"
	}
	return msg
}

func exitCode(err error) int {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return -1
}
