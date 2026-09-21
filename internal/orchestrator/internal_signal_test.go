package orchestrator

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
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

	// 사람에게 보일 오류와 바깥 오류는 건드리지 않는다.
	facing := newHumanFacing("빌드가 오류 15개로 깨졌다")
	wrapped := fmt.Errorf("감싼다: %w", facing)
	if tc.humanReason(wrapped) != wrapped {
		t.Errorf("사람에게 보일 오류를 걷어 냈다")
	}
	plain := errors.New("바깥에서 온 오류")
	if tc.humanReason(plain) != plain {
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

// 마지막 관문은 execute 의 **첫 문장**인 defer 여야 하고, 그 defer 의 몸통
// **맨 위**에서 이름 붙은 오류 자신을 humanReason 에 넘겨 되받아야 한다.
//
// 느슨하게 보면 죽은 채로 초록이 된다 — `if false { err = t.humanReason(err) }`
// 는 글자도 나무도 남고, `err = t.humanReason(nil)` 은 모든 실패를 성공으로
// 바꾼다. 둘 다 실제로 통과하던 모양이다.
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
	errName := namedErrorResult(fn)
	if errName == "" {
		t.Fatal("execute 의 오류 반환값에 이름이 없다 — defer 가 바꿀 수 없다")
	}
	if len(fn.Body.List) == 0 {
		t.Fatal("execute 의 몸통이 비었다")
	}

	d, ok := fn.Body.List[0].(*ast.DeferStmt)
	if !ok {
		t.Fatalf("execute 의 첫 문장이 defer 가 아니다 — 그 앞의 return 은 관문을 지나지 않는다")
	}
	lit, ok := d.Call.Fun.(*ast.FuncLit)
	if !ok {
		t.Fatal("관문 defer 가 함수 리터럴이 아니다")
	}
	for _, st := range lit.Body.List { // 맨 위 문장만 본다. 조건 안은 죽을 수 있다.
		if isHumanReasonRoundTrip(st, errName) {
			return
		}
	}
	t.Errorf("관문 defer 의 맨 위에 %s = …humanReason(%s) 가 없다", errName, errName)
}

// isHumanReasonRoundTrip 은 err = ….humanReason(err) 인지 본다.
// 넘기는 것이 그 오류 자신이어야 한다 — nil 을 넘기면 실패가 통째로 사라진다.
func isHumanReasonRoundTrip(st ast.Stmt, errName string) bool {
	as, ok := st.(*ast.AssignStmt)
	if !ok || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
		return false
	}
	lhs, ok := as.Lhs[0].(*ast.Ident)
	if !ok || lhs.Name != errName {
		return false
	}
	call, ok := as.Rhs[0].(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "humanReason" {
		return false
	}
	arg, ok := call.Args[0].(*ast.Ident)
	return ok && arg.Name == errName
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

// 이 꾸러미의 오류는 두 만들개로만 만든다.
//
// 이름으로 거르면(err… 로 시작하는 것만) 이름만 바꿔 빠져나간다. 목록으로
// 갈라도 목록에 적는 것이 곧 방어를 끄는 스위치가 된다. 만드는 자리를
// 막으면 둘 다 없어진다.
func TestErrorsAreMadeByAConstructor(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	var bad []string
	for _, pkg := range pkgs {
		errTypes := errorTypesIn(pkg)
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
					for i, id := range vs.Names {
						if i >= len(vs.Values) {
							continue
						}
						if why := notAConstructor(vs.Values[i], errTypes); why != "" {
							bad = append(bad, fmt.Sprintf("%s 의 %s — %s", shortName(name), id.Name, why))
						}
					}
				}
			}
		}
	}
	sort.Strings(bad)
	for _, b := range bad {
		t.Errorf("%s. 이 꾸러미의 오류는 newInternalSignal(흐름 안쪽) 이나 newHumanFacing(사람이 볼 문구) 으로 만든다", b)
	}
}

// errorTypesIn 은 이 꾸러미에서 Error() 를 가진 타입 이름을 모은다.
func errorTypesIn(pkg *ast.Package) map[string]bool {
	out := map[string]bool{}
	for _, f := range pkg.Files {
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || fn.Name.Name != "Error" || len(fn.Recv.List) != 1 {
				continue
			}
			switch rt := fn.Recv.List[0].Type.(type) {
			case *ast.StarExpr:
				if id, ok := rt.X.(*ast.Ident); ok {
					out[id.Name] = true
				}
			case *ast.Ident:
				out[rt.Name] = true
			}
		}
	}
	return out
}

// notAConstructor 는 그 값이 오류를 만드는데 만들개를 안 쓴 경우 까닭을 준다.
func notAConstructor(v ast.Expr, errTypes map[string]bool) string {
	switch e := v.(type) {
	case *ast.CallExpr:
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok {
			if pkg, ok := sel.X.(*ast.Ident); ok {
				if (pkg.Name == "errors" && sel.Sel.Name == "New") ||
					(pkg.Name == "fmt" && sel.Sel.Name == "Errorf") {
					return pkg.Name + "." + sel.Sel.Name + " 으로 만들었다"
				}
			}
		}
	case *ast.UnaryExpr:
		lit, ok := e.X.(*ast.CompositeLit)
		if !ok {
			return ""
		}
		if id, ok := lit.Type.(*ast.Ident); ok && errTypes[id.Name] {
			return "오류 타입 " + id.Name + " 을 그 자리에서 만들었다"
		}
	case *ast.CompositeLit:
		if id, ok := e.Type.(*ast.Ident); ok && errTypes[id.Name] {
			return "오류 타입 " + id.Name + " 을 그 자리에서 만들었다"
		}
	}
	return ""
}

func shortName(path string) string {
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
}
