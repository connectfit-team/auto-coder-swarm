package orchestrator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// TypeScript·Svelte 도 파싱한다.
//
// tsc 를 그대로 돌리면 없는 모듈까지 오류로 낸다 — 의존성을 받아 두지 않았다.
// 파서만 부르는 작은 스크립트를 둔다(~/tools/tsparse/parse.js).
// .svelte 는 스크립트와 마크업을 나눠 각각 맞는 파서로 본다 — Svelte 의 파서는
// 스크립트를 JS 로 읽어서 lang="ts" 의 타입 표기를 문법 오류로 낸다.
//
// 맞으면 0, 틀리면 1 로 끝나고 까닭을 찍는다.

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
// 파서가 없으면 판단하지 않는다(빈 문자열).
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
	msg := strings.TrimSpace(string(b))
	if msg == "" {
		return ""
	}
	return msg
}
