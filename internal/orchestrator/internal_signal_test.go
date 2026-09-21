package orchestrator

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
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

	real := fmt.Errorf("빌드가 오류 15개로 깨졌다")
	if tc.humanReason(real) != real {
		t.Errorf("바깥 오류를 바꿨다")
	}
	if tc.humanReason(nil) != nil {
		t.Errorf("nil 을 오류로 바꿨다")
	}
}

// 사람이 봐도 되는 목록은 관문을 끄는 스위치가 아니다.
func TestHumanReadableListCannotHideASignal(t *testing.T) {
	tc := &taskContext{}
	for _, sig := range humanReadableSignals {
		var marked *internalSignal
		if errors.As(sig, &marked) {
			t.Errorf("안쪽 신호가 사람이 봐도 되는 목록에 있다: %v", sig)
			continue
		}
		wrapped := fmt.Errorf("감싼다: %w", sig)
		if got := tc.humanReason(wrapped); got != wrapped {
			t.Errorf("사람이 봐도 되는 신호를 걷어 냈다: %v → %v", sig, got)
		}
	}
	// 목록에 적어도 표식이 있으면 가려진다.
	hidden := newInternalSignal("가려져야 한다")
	if got := tc.humanReason(hidden).Error(); strings.Contains(got, "가려져야 한다") {
		t.Errorf("표식이 있는데 그대로 나갔다: %q", got)
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

// 마지막 관문은 execute 의 **맨 위 defer** 여야 한다.
//
// 글자 대조로는 못 본다. 주석으로 남기거나 `if false { … }` 안에 넣어도
// 글자는 그대로라 통과한다 — 관문이 죽었는데 초록이다.
func TestLastGateIsWired(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "flow.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	fn := findMethod(f, "execute")
	if fn == nil {
		t.Fatal("flow.go 에서 execute 를 못 찾았다")
	}

	// 이름 붙은 반환값이라야 defer 가 바꿀 수 있다.
	errName := namedErrorResult(fn)
	if errName == "" {
		t.Fatal("execute 의 오류 반환값에 이름이 없다 — defer 가 바꿀 수 없다")
	}

	for _, st := range fn.Body.List {
		d, ok := st.(*ast.DeferStmt)
		if !ok {
			continue
		}
		lit, ok := d.Call.Fun.(*ast.FuncLit)
		if !ok {
			continue
		}
		if assignsHumanReason(lit.Body, errName) {
			return
		}
	}
	t.Errorf("execute 의 몸통 맨 위에 %s = t.humanReason(%s) 를 하는 defer 가 없다", errName, errName)
}

func findMethod(f *ast.File, name string) *ast.FuncDecl {
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if ok && fn.Recv != nil && fn.Name.Name == name && fn.Body != nil {
			return fn
		}
	}
	return nil
}

// namedErrorResult 는 이름 붙은 error 반환값의 이름을 준다.
func namedErrorResult(fn *ast.FuncDecl) string {
	if fn.Type.Results == nil {
		return ""
	}
	for _, r := range fn.Type.Results.List {
		id, ok := r.Type.(*ast.Ident)
		if !ok || id.Name != "error" || len(r.Names) != 1 {
			continue
		}
		return r.Names[0].Name
	}
	return ""
}

// assignsHumanReason 은 그 몸통이 err = t.humanReason(err) 를 하는지 본다.
func assignsHumanReason(body *ast.BlockStmt, errName string) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
			return true
		}
		lhs, ok := as.Lhs[0].(*ast.Ident)
		if !ok || lhs.Name != errName {
			return true
		}
		call, ok := as.Rhs[0].(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if ok && sel.Sel.Name == "humanReason" {
			found = true
		}
		return true
	})
	return found
}

// 신호를 새로 만들었으면 표식을 달거나 사람이 봐도 된다고 적어야 한다.
//
// 훑는 범위는 **이 꾸러미가 실제로 의존하는 꾸러미**다. execute 는 그것들의
// 오류를 그대로 올려 보내므로 그것들만 사람 사유가 될 수 있다. 이 꾸러미를
// 쓰는 쪽(internal/api·cmd)까지 훑으면 등록할 방법이 없는 막다른 길이 된다 —
// 등록하면 되돌이 import 가 되고, 숨기면 캡슐화를 깨라는 말이 된다.
func TestEverySentinelIsAccountedFor(t *testing.T) {
	human := registeredHumanReadable(t)

	for _, s := range declaredSignals(t) {
		if s.marked || human[s.name] {
			continue
		}
		t.Errorf("%s 의 %s 가 갈리지 않았다 — 안쪽 신호면 newInternalSignal 로 만들고, 사람이 봐도 되면 humanReadableSignals 에 넣어라",
			s.pkg, s.name)
	}
}

// registeredHumanReadable 는 humanReadableSignals 의 원소 이름을 준다.
func registeredHumanReadable(t *testing.T) map[string]bool {
	t.Helper()
	out := map[string]bool{}
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
			if !ok || len(vs.Names) != 1 || len(vs.Values) != 1 || vs.Names[0].Name != "humanReadableSignals" {
				continue
			}
			lit, ok := vs.Values[0].(*ast.CompositeLit)
			if !ok {
				t.Fatal("humanReadableSignals 가 목록 리터럴이 아니다")
			}
			for _, e := range lit.Elts {
				if n := signalName(e); n != "" {
					out[n] = true
				}
			}
		}
	}
	if len(out) == 0 {
		t.Fatal("humanReadableSignals 를 못 읽었다")
	}
	return out
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

type sentinel struct {
	pkg    string
	name   string // 이 꾸러미 것은 그냥 이름, 다른 꾸러미 것은 pkg.Name
	marked bool   // newInternalSignal 로 만들었다
}

// declaredSignals 는 이 꾸러미와 그것이 의존하는 꾸러미의 err… 변수를 준다.
func declaredSignals(t *testing.T) []sentinel {
	t.Helper()
	var out []sentinel
	for _, dir := range dependencyDirs(t) {
		fset := token.NewFileSet()
		pkgs, err := parser.ParseDir(fset, dir, func(fi fs.FileInfo) bool {
			return !strings.HasSuffix(fi.Name(), "_test.go")
		}, 0)
		if err != nil {
			t.Fatalf("%s 를 못 읽었다: %v", dir, err)
		}
		for _, pkg := range pkgs {
			own := pkg.Name == "orchestrator"
			for _, f := range pkg.Files {
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
						for i, id := range vs.Names {
							if !strings.HasPrefix(strings.ToLower(id.Name), "err") {
								continue
							}
							// 밖에서 가리킬 수 없는 값은 이 꾸러미의 목록에
							// 올릴 수 없다. 지킬 방법이 없는 것을 요구하지 않는다.
							if !own && !ast.IsExported(id.Name) {
								continue
							}
							name := id.Name
							if !own {
								name = pkg.Name + "." + id.Name
							}
							out = append(out, sentinel{
								pkg:    pkg.Name,
								name:   name,
								marked: own && madeByNewInternalSignal(vs, i),
							})
						}
					}
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

func madeByNewInternalSignal(vs *ast.ValueSpec, i int) bool {
	if i >= len(vs.Values) {
		return false
	}
	call, ok := vs.Values[i].(*ast.CallExpr)
	if !ok {
		return false
	}
	id, ok := call.Fun.(*ast.Ident)
	return ok && id.Name == "newInternalSignal"
}

// dependencyDirs 는 이 꾸러미가 (곧바로든 건너서든) 들여오는 이 저장소 안
// 꾸러미의 디렉터리를 준다. 자기 자신도 넣는다.
func dependencyDirs(t *testing.T) []string {
	t.Helper()
	root := filepath.Join("..", "..")
	mod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	var prefix string
	for _, line := range strings.Split(string(mod), "\n") {
		if strings.HasPrefix(line, "module ") {
			prefix = strings.TrimSpace(strings.TrimPrefix(line, "module ")) + "/"
			break
		}
	}
	if prefix == "" {
		t.Fatal("go.mod 에서 모듈 이름을 못 읽었다")
	}

	seen := map[string]bool{".": true}
	queue := []string{"."}
	for len(queue) > 0 {
		dir := queue[0]
		queue = queue[1:]
		fset := token.NewFileSet()
		pkgs, err := parser.ParseDir(fset, dir, func(fi fs.FileInfo) bool {
			return !strings.HasSuffix(fi.Name(), "_test.go")
		}, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("%s 를 못 읽었다: %v", dir, err)
		}
		for _, pkg := range pkgs {
			for _, f := range pkg.Files {
				for _, im := range f.Imports {
					path := strings.Trim(im.Path.Value, `"`)
					if !strings.HasPrefix(path, prefix) {
						continue
					}
					next := filepath.Join(root, strings.TrimPrefix(path, prefix))
					if seen[next] {
						continue
					}
					seen[next] = true
					queue = append(queue, next)
				}
			}
		}
	}
	dirs := make([]string, 0, len(seen))
	for d := range seen {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	return dirs
}
