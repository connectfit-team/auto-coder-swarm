package orchestrator

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
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
	if !strings.Contains(got, "되먹임") {
		t.Errorf("되먹임을 사람에게 하는 말처럼 붙였다: %q", got)
	}
	if strings.Contains(got, "번 고쳐") {
		t.Errorf("관문이 시도 횟수를 지어냈다: %q", got)
	}

	if bare := (&taskContext{}).humanReason(errRetryPlanning).Error(); strings.Contains(bare, errRetryPlanning.Error()) {
		t.Errorf("되먹임이 없을 때 안쪽 신호가 그대로 나갔다: %q", bare)
	}

	// 사람이 봐도 되는 신호는 문구가 그대로 남아야 한다.
	for _, sig := range humanReadableSignals {
		wrapped := fmt.Errorf("감싼다: %w", sig)
		if got := tc.humanReason(wrapped); got != wrapped {
			t.Errorf("사람이 봐도 되는 신호를 걷어 냈다: %v → %v", sig, got)
		}
	}

	real := fmt.Errorf("빌드가 오류 15개로 깨졌다")
	if tc.humanReason(real) != real {
		t.Errorf("바깥 오류를 바꿨다")
	}
	if tc.humanReason(nil) != nil {
		t.Errorf("nil 을 오류로 바꿨다")
	}
}

func TestWhyKeptRetryingReadsRight(t *testing.T) {
	tc := &taskContext{lastFeedback: "PLAN REJECTED: 그 경로는 이 저장소에 없다"}
	if got := tc.whyKeptRetrying("계획", 3).Error(); !strings.HasPrefix(got, "계획을 3번 고쳐 봤지만") {
		t.Errorf("조사나 횟수가 틀렸다: %q", got)
	}
	if got := tc.whyKeptRetrying("코딩", 2).Error(); !strings.HasPrefix(got, "코딩을 2번") {
		t.Errorf("횟수를 손으로 박아 두었다: %q", got)
	}
	if got := (&taskContext{}).whyKeptRetrying("계획", 3).Error(); !strings.Contains(got, "까닭이 기록되지 않았다") {
		t.Errorf("되먹임이 없는 경우를 갈라 적지 않았다: %q", got)
	}
}

// 신호를 보는 자리는 모두 사유로 바꿔야 하고, 마지막 관문이 걸려 있어야 한다.
func TestLastGateIsWired(t *testing.T) {
	src := readSource(t, "flow.go")
	if !strings.Contains(src, "err = t.humanReason(err)") {
		t.Error("execute 에 humanReason 관문이 없다")
	}
	sees := strings.Count(src, "errors.Is(err, errRetryPlanning)")
	turns := strings.Count(src, "t.whyKeptRetrying(")
	if sees == 0 || sees != turns {
		t.Errorf("신호를 보는 자리 %d 곳, 사유로 바꾸는 곳 %d 곳 — 같아야 한다", sees, turns)
	}
}

// 신호를 새로 만들었으면 두 목록 가운데 하나의 원소여야 한다.
//
// 글자 대조로는 못 잡는다. 이름이 겹치기만 해도(errRetry ⊂ errRetryPlanning)
// 통과하고, 주석에 이름만 적어도 통과한다. 두 파일 다 문법 나무로 읽는다.
func TestEverySentinelIsAccountedFor(t *testing.T) {
	internal, human := registeredSignals(t)
	for name := range internal {
		if human[name] {
			t.Errorf("%s 가 두 목록에 다 있다 — 한 쪽만 골라라", name)
		}
	}

	for _, s := range declaredSignals(t, "..") {
		if !internal[s] && !human[s] {
			t.Errorf("%s 가 internal_signal.go 의 어느 목록에도 없다 — 안쪽 신호면 internalSignals 에, 사람이 봐도 되면 humanReadableSignals 에 넣어라", s)
		}
	}
}

// registeredSignals 는 두 목록의 원소 이름을 준다. 다른 꾸러미 것은 pkg.Name 이다.
func registeredSignals(t *testing.T) (internal, human map[string]bool) {
	t.Helper()
	internal, human = map[string]bool{}, map[string]bool{}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "internal_signal.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.VAR {
			continue
		}
		for _, sp := range gd.Specs {
			vs, ok := sp.(*ast.ValueSpec)
			if !ok || len(vs.Names) != 1 || len(vs.Values) != 1 {
				continue
			}
			var into map[string]bool
			switch vs.Names[0].Name {
			case "internalSignals":
				into = internal
			case "humanReadableSignals":
				into = human
			default:
				continue
			}
			lit, ok := vs.Values[0].(*ast.CompositeLit)
			if !ok {
				t.Fatalf("%s 가 목록 리터럴이 아니다", vs.Names[0].Name)
			}
			for _, e := range lit.Elts {
				if n := signalName(e); n != "" {
					into[n] = true
				}
			}
		}
	}
	if len(internal) == 0 {
		t.Fatal("internalSignals 가 비었다 — 목록을 못 읽었다")
	}
	return internal, human
}

// signalName 은 목록 원소의 이름을 준다. x 이거나 pkg.X 다.
func signalName(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		if p, ok := v.X.(*ast.Ident); ok {
			return p.Name + "." + v.Sel.Name
		}
	}
	return ""
}

// declaredSignals 는 root 아래 모든 꾸러미의 최상위 err·Err 변수를 준다.
//
// 이 꾸러미 것은 그냥 이름, 다른 꾸러미 것은 pkg.Name 이다 — execute 가
// 다른 단계의 오류를 그대로 올려 보내므로 그것들도 사람 사유가 될 수 있다.
func declaredSignals(t *testing.T, root string) []string {
	t.Helper()
	seen := map[string]bool{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		if n := d.Name(); n == "testdata" || n == "vendor" {
			return filepath.SkipDir
		}
		fset := token.NewFileSet()
		pkgs, err := parser.ParseDir(fset, p, func(fi fs.FileInfo) bool {
			return !strings.HasSuffix(fi.Name(), "_test.go")
		}, 0)
		if err != nil {
			return nil
		}
		for _, pkg := range pkgs {
			for _, f := range pkg.Files {
				for _, dcl := range f.Decls {
					gd, ok := dcl.(*ast.GenDecl)
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
							name := id.Name
							if pkg.Name != "orchestrator" {
								if !ast.IsExported(id.Name) {
									t.Errorf("%s 꾸러미의 %s 는 밖에서 가리킬 수 없다 — 목록에 넣을 수 있게 내보내거나, 밖으로 내보내지 마라", pkg.Name, id.Name)
									continue
								}
								name = pkg.Name + "." + id.Name
							}
							seen[name] = true
						}
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
