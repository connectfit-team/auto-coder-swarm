package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sheetRepo 는 서로 다른 모듈을 들여오는 파일 n개짜리 사본을 만든다.
//
// AvailableNames 는 그 파일의 **import** 를 보고 「무엇을 쓸 수 있나」 를
// 만든다. 들여오는 것이 없으면 아무것도 안 나온다 — 시험 자료가 그래서
// 헛통과하기 쉽다.
func sheetRepo(t *testing.T, n, exportsPerLib int) (string, []string) {
	t.Helper()
	dir := t.TempDir()
	var files []string
	for i := 0; i < n; i++ {
		lib := fmt.Sprintf("lib%d.ts", i)
		var body strings.Builder
		for j := 0; j < exportsPerLib; j++ {
			fmt.Fprintf(&body, "export const MARK%d_%d = %d;\n", i, j, j)
		}
		if err := os.WriteFile(filepath.Join(dir, lib), []byte(body.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		use := fmt.Sprintf("use%d.ts", i)
		src := fmt.Sprintf("import { MARK%d_0 } from './lib%d';\n", i, i)
		if err := os.WriteFile(filepath.Join(dir, use), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		files = append(files, use)
	}
	return dir, files
}

// 앞의 3개 파일만 보던 것이 틀린 답의 원인이었다.
//
// 실측: 같은 요청에 목록이 1,197자(틀림)와 3,193자(맞음)로 갈렸고, **틀린
// 판의 목록은 맞은 판의 부분집합**이었다 — 틀린 판에만 있는 줄이 0개다.
// 회차마다 files 의 차례가 달라 연결 관련 파일이 3개 안에 못 들어온 것이다.
func TestStateSheetGoesBeyondThreeFiles(t *testing.T) {
	dir, files := sheetRepo(t, 8, 2)
	tc := &taskContext{repoPath: dir}
	got := tc.stateSheet(files)
	if got == "" {
		t.Fatal("목록이 비었다 — 이 시험이 아무것도 재지 못한다")
	}
	seen := 0
	for i := 0; i < 8; i++ {
		if strings.Contains(got, fmt.Sprintf("MARK%d_", i)) {
			seen++
		}
	}
	if seen <= 3 {
		t.Fatalf("아직 3개까지만 본다 (%d개 보임):\n%s", seen, got)
	}
}

// 끝없이 담지는 않는다 — 창을 넘기면 물음 자체가 실패한다.
func TestStateSheetStaysBounded(t *testing.T) {
	dir, files := sheetRepo(t, 40, 400)
	tc := &taskContext{repoPath: dir}
	got := tc.stateSheet(files)
	if got == "" {
		t.Fatal("목록이 비었다 — 이 시험이 아무것도 재지 못한다")
	}
	// 한 파일을 다 쓴 뒤에야 끊으므로 상한을 조금 넘을 수 있다. 두 배는 안 넘는다.
	if len(got) > stateSheetChars*2 {
		t.Fatalf("너무 많이 담았다: %d자 (상한 %d)", len(got), stateSheetChars)
	}
	// 그리고 파일 수 상한도 지킨다.
	seen := 0
	for i := 0; i < 40; i++ {
		if strings.Contains(got, fmt.Sprintf("MARK%d_", i)) {
			seen++
		}
	}
	if seen > stateSheetFiles {
		t.Fatalf("파일 상한을 넘겼다: %d개 (상한 %d)", seen, stateSheetFiles)
	}
}

func TestStateSheetEmptyInput(t *testing.T) {
	tc := &taskContext{repoPath: t.TempDir()}
	if got := tc.stateSheet(nil); got != "" {
		t.Fatalf("빈 입력에 무언가 냈다: %q", got)
	}
}
