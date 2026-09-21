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

// 마지막 관문은 execute 의 **첫 문장**인 defer 여야 하고, 그 defer 의 몸통은
// 이름 붙은 오류 자신을 humanReason 에 넘겨 되받는 **한 문장뿐**이어야 한다.
//
// 느슨하게 보면 죽은 채로 초록이 된다. 실제로 이런 것들이 통과했다 —
// `if false { … }`, `humanReason(nil)`, 앞에 `err = nil` 한 줄, 앞에 이른
// return, 뒤에 되돌리기, 다른 수신자의 같은 이름 메서드.
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
	recv := receiverName(fn)
	if recv == "" {
		t.Fatal("execute 의 수신자에 이름이 없다")
	}
	if len(fn.Body.List) == 0 {
		t.Fatal("execute 의 몸통이 비었다")
	}

	d, ok := fn.Body.List[0].(*ast.DeferStmt)
	if !ok {
		t.Fatal("execute 의 첫 문장이 defer 가 아니다 — 그 앞의 return 은 관문을 지나지 않는다")
	}
	lit, ok := d.Call.Fun.(*ast.FuncLit)
	if !ok {
		t.Fatal("관문 defer 가 함수 리터럴이 아니다")
	}
	// **한 문장뿐이어야 한다.** 앞에 무엇을 두면 건너뛸 수 있고, 뒤에 무엇을
	// 두면 되돌릴 수 있다.
	if len(lit.Body.List) != 1 {
		t.Fatalf("관문 defer 의 몸통이 %d 문장이다 — %s = %s.humanReason(%s) 한 문장뿐이어야 한다",
			len(lit.Body.List), errName, recv, errName)
	}
	if !isHumanReasonRoundTrip(lit.Body.List[0], errName, recv) {
		t.Errorf("관문 defer 가 %s = %s.humanReason(%s) 가 아니다", errName, recv, errName)
	}
}

// isHumanReasonRoundTrip 은 err = t.humanReason(err) 인지 본다.
//
// 넘기는 것이 그 오류 자신이어야 하고(nil 을 넘기면 실패가 통째로 사라진다),
// 부르는 대상이 그 메서드의 수신자여야 한다(같은 이름의 항등 메서드를 가진
// 다른 타입을 놓으면 관문이 사라진다).
func isHumanReasonRoundTrip(st ast.Stmt, errName, recv string) bool {
	as, ok := st.(*ast.AssignStmt)
	if !ok || as.Tok != token.ASSIGN || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
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
	x, ok := sel.X.(*ast.Ident)
	if !ok || x.Name != recv {
		return false
	}
	arg, ok := call.Args[0].(*ast.Ident)
	return ok && arg.Name == errName
}

// receiverName 은 메서드 수신자의 이름을 준다.
func receiverName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) != 1 || len(fn.Recv.List[0].Names) != 1 {
		return ""
	}
	return fn.Recv.List[0].Names[0].Name
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
// 이름으로 거르면 이름만 바꿔 빠져나가고, 목록으로 갈래를 나누면 목록에 적는
// 것이 곧 방어를 끄는 스위치가 된다. 만드는 자리를 막으면 둘 다 없어진다.
//
// **막는 범위는 이것이다** — 꾸러미 수준 오류 변수와 오류 타입 선언. 함수
// 안에서 그 자리에 만들어 돌려주는 오류(errors.New·fmt.Errorf)는 막지 않는다.
// 그것들은 대개 사람이 읽을 사유이고, 흐름 갈래로 쓰려면 같은 값을 다시
// 가리켜야 하므로 결국 꾸러미 수준 변수나 타입이 필요하다.
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
		// 오류 타입은 둘뿐이다. 새로 만들면 갈래가 값에 박히지 않는다.
		for name, where := range errorTypesIn(pkg) {
			if name == "internalSignal" || name == "humanFacing" {
				continue
			}
			bad = append(bad, fmt.Sprintf("%s 의 오류 타입 %s — 새 오류 타입을 만들었다", shortName(where), name))
		}
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
						if why := notAConstructor(id.Name, vs.Values[i]); why != "" {
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

// errorTypesIn 은 이 꾸러미에서 Error() 를 가진 타입 이름과 그 파일을 준다.
func errorTypesIn(pkg *ast.Package) map[string]string {
	out := map[string]string{}
	for path, f := range pkg.Files {
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || fn.Name.Name != "Error" || len(fn.Recv.List) != 1 {
				continue
			}
			switch rt := fn.Recv.List[0].Type.(type) {
			case *ast.StarExpr:
				if id, ok := rt.X.(*ast.Ident); ok {
					out[id.Name] = path
				}
			case *ast.Ident:
				out[rt.Name] = path
			}
		}
	}
	return out
}

// notAConstructor 는 그 변수가 만들개를 안 쓴 경우 까닭을 준다.
//
// 이름이 err 로 시작하면 무엇으로 만들었든 곧바로 만들개여야 한다 — 한 겹
// 감싼 함수로 만들면 갈래가 값에 박히지 않는다.
func notAConstructor(name string, v ast.Expr) string {
	if call, ok := v.(*ast.CallExpr); ok {
		if id, ok := call.Fun.(*ast.Ident); ok &&
			(id.Name == "newInternalSignal" || id.Name == "newHumanFacing") {
			return ""
		}
	}
	if strings.HasPrefix(strings.ToLower(name), "err") {
		return "만들개로 만들지 않았다"
	}
	if sel, ok := callSelector(v); ok &&
		((sel[0] == "errors" && sel[1] == "New") || (sel[0] == "fmt" && sel[1] == "Errorf")) {
		return sel[0] + "." + sel[1] + " 으로 만들었다"
	}
	return ""
}

// callSelector 는 pkg.Func(…) 꼴이면 그 둘을 준다.
func callSelector(v ast.Expr) ([2]string, bool) {
	call, ok := v.(*ast.CallExpr)
	if !ok {
		return [2]string{}, false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return [2]string{}, false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return [2]string{}, false
	}
	return [2]string{pkg.Name, sel.Sel.Name}, true
}

func shortName(path string) string {
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
}
