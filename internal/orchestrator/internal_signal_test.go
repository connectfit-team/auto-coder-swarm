package orchestrator

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestInternalSignalNeverReachesHuman(t *testing.T) {
	tc := &taskContext{lastFeedback: "CODER FAILED: a.ts 를 못 고쳤다. 다시 계획하라."}

	got := tc.humanReason(fmt.Errorf("시도 3: %w", errRetryPlanning)).Error()
	if strings.Contains(got, errRetryPlanning.Error()) {
		t.Errorf("안쪽 신호가 그대로 사유가 됐다: %q", got)
	}
	if !strings.Contains(got, "a.ts 를 못 고쳤다") {
		t.Errorf("왜 막혔는지가 빠졌다: %q", got)
	}
	// 되먹임은 모델에게 쓴 말이다. 누구에게 한 말인지 밝혀야 한다.
	if !strings.Contains(got, "되먹임") {
		t.Errorf("되먹임을 사람에게 하는 말처럼 붙였다: %q", got)
	}
	// 관문은 몇 번째 시도인지 모른다 — 지어내면 안 된다.
	if strings.Contains(got, "번 고쳐") {
		t.Errorf("관문이 시도 횟수를 지어냈다: %q", got)
	}

	if bare := (&taskContext{}).humanReason(errRetryPlanning).Error(); strings.Contains(bare, errRetryPlanning.Error()) {
		t.Errorf("되먹임이 없을 때 안쪽 신호가 그대로 나갔다: %q", bare)
	}

	real := fmt.Errorf("빌드가 오류 15개로 깨졌다")
	if tc.humanReason(real) != real {
		t.Errorf("바깥 오류를 바꿨다")
	}
	if tc.humanReason(nil) != nil {
		t.Errorf("nil 을 오류로 바꿨다")
	}
}

// 사유의 조사와 횟수가 사실과 맞아야 한다.
func TestWhyKeptRetryingReadsRight(t *testing.T) {
	tc := &taskContext{lastFeedback: "PLAN REJECTED: 그 경로는 이 저장소에 없다"}
	got := tc.whyKeptRetrying("계획", 3).Error()
	if !strings.HasPrefix(got, "계획을 3번 고쳐 봤지만") {
		t.Errorf("조사나 횟수가 틀렸다: %q", got)
	}
	if got := tc.whyKeptRetrying("코딩", 2).Error(); !strings.HasPrefix(got, "코딩을 2번") {
		t.Errorf("횟수를 손으로 박아 두었다: %q", got)
	}
	empty := (&taskContext{}).whyKeptRetrying("계획", 3).Error()
	if !strings.Contains(empty, "까닭이 기록되지 않았다") {
		t.Errorf("되먹임이 없는 경우를 갈라 적지 않았다: %q", empty)
	}
}

// 마지막 관문은 execute 에 걸려 있어야 하고, 두 자리 모두 사유로 바꿔야 한다.
func TestLastGateIsWired(t *testing.T) {
	src := readSource(t, "flow.go")
	if !strings.Contains(src, "err = t.humanReason(err)") {
		t.Error("execute 에 humanReason 관문이 없다")
	}
	if n := strings.Count(src, "errRetryPlanning"); n != strings.Count(src, "t.whyKeptRetrying(")+1 {
		t.Errorf("신호를 보는 자리 %d 곳 가운데 사유로 바꾸지 않는 곳이 있다", n)
	}
}

// 신호를 새로 만들었으면 internal_signal.go 에 적혀 있어야 한다.
//
// 글자 대조(정규식)로는 못 잡는다. 묶음 선언 `var ( errX = … )`, 대문자
// Err 접두, errors.New 가 아닌 사용자 정의 타입이 전부 빠져나갔다 — 실제로
// 같은 꾸러미의 errChangedTestsFailed · errNewTypeErrors 가 그렇게 빠져
// 있었다. 문법 나무로 읽는다.
func TestEverySentinelIsAccountedFor(t *testing.T) {
	reg := readSource(t, "internal_signal.go")

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range pkgs {
		for name, f := range pkg.Files {
			for _, d := range f.Decls {
				gd, ok := d.(*ast.GenDecl)
				if !ok || gd.Tok != token.VAR {
					continue
				}
				for _, sp := range gd.Specs {
					vs, ok := sp.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for _, id := range vs.Names {
						if !strings.HasPrefix(strings.ToLower(id.Name), "err") {
							continue
						}
						if !strings.Contains(reg, id.Name) {
							t.Errorf("%s 의 %s 가 internal_signal.go 에 없다 — 안쪽 신호면 internalSignals 에, 사람이 봐도 되면 humanReadableSignals 에 적어라",
								filepath.Base(name), id.Name)
						}
					}
				}
			}
		}
	}
}
