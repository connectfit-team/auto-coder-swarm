package agent

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"
)

// 파일의 뼈대를 함께 보여 준다.
//
// 코더는 "지시문에 나온 이름이 있는 줄 둘레" 만 본다. 볼 것을 줄이려고
// 그렇게 했는데, 그 창에 **구조체 선언과 이미 있는 메서드가 안 들어온다.**
// 그래서 없는 필드와 이미 있는 이름을 지어낸다. 실측(W-49301):
//
//	c.repo undefined (type *Connect has no field or method repo)
//	  → 진짜 이름은 다른 것이다
//	UpdateCEOWorkConnectState 를 찾으라고 했다
//	  → 이미 CEOWorkConnectUpdateState 가 있다(낱말 6/6 겹침)
//
// 뼈대는 짧다 — 타입과 그 필드, 그 타입의 메서드 이름, 그 밖의 함수 이름.
// 그것만 있으면 필드도 메서드도 지어낼 이유가 없다.

const outlineMaxRunes = 2500

// FileOutline 은 그 파일의 최상위 선언을 짧게 적는다.
// Go 가 아니거나 파싱이 안 되면 빈 문자열이다.
func FileOutline(path, content string) string {
	if !strings.HasSuffix(path, ".go") {
		return ""
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, content, parser.SkipObjectResolution)
	if err != nil {
		return "" // 뭉개진 파일의 뼈대는 믿을 수 없다
	}

	types := map[string][]string{}   // 타입 → 필드
	methods := map[string][]string{} // 타입 → 메서드
	var funcs, consts []string

	for _, d := range f.Decls {
		switch n := d.(type) {
		case *ast.GenDecl:
			for _, sp := range n.Specs {
				switch s := sp.(type) {
				case *ast.TypeSpec:
					types[s.Name.Name] = fieldNames(s.Type)
				case *ast.ValueSpec:
					for _, id := range s.Names {
						consts = append(consts, id.Name)
					}
				}
			}
		case *ast.FuncDecl:
			if n.Recv != nil && len(n.Recv.List) > 0 {
				recv := recvType(n.Recv.List[0].Type)
				methods[recv] = append(methods[recv], n.Name.Name)
				continue
			}
			funcs = append(funcs, n.Name.Name)
		}
	}

	var b strings.Builder
	b.WriteString("[이 파일의 뼈대 — 여기 없는 필드·메서드를 지어내지 마라]\n")
	for _, t := range sortedNames(types) {
		if fs := types[t]; len(fs) > 0 {
			fmt.Fprintf(&b, "type %s { %s }\n", t, strings.Join(fs, ", "))
		} else {
			fmt.Fprintf(&b, "type %s\n", t)
		}
		if ms := methods[t]; len(ms) > 0 {
			sort.Strings(ms)
			fmt.Fprintf(&b, "  메서드: %s\n", strings.Join(ms, ", "))
		}
	}
	// 받는이가 이 파일에 없는 타입의 메서드도 있다(다른 파일의 타입).
	for _, t := range sortedNames(methods) {
		if _, ok := types[t]; ok {
			continue
		}
		ms := methods[t]
		sort.Strings(ms)
		fmt.Fprintf(&b, "%s 의 메서드: %s\n", t, strings.Join(ms, ", "))
	}
	if len(funcs) > 0 {
		sort.Strings(funcs)
		fmt.Fprintf(&b, "함수: %s\n", strings.Join(funcs, ", "))
	}
	if len(consts) > 0 {
		sort.Strings(consts)
		fmt.Fprintf(&b, "상수·변수: %s\n", strings.Join(consts, ", "))
	}

	out := b.String()
	if r := []rune(out); len(r) > outlineMaxRunes {
		out = string(r[:outlineMaxRunes]) + "\n… (뒤는 생략)\n"
	}
	return out
}

func fieldNames(t ast.Expr) []string {
	var out []string
	switch n := t.(type) {
	case *ast.StructType:
		for _, f := range n.Fields.List {
			if len(f.Names) == 0 {
				out = append(out, exprName(f.Type)) // 묻어 넣은 것
				continue
			}
			for _, id := range f.Names {
				out = append(out, id.Name)
			}
		}
	case *ast.InterfaceType:
		for _, m := range n.Methods.List {
			for _, id := range m.Names {
				out = append(out, id.Name+"()")
			}
		}
	}
	return out
}

func recvType(t ast.Expr) string {
	if s, ok := t.(*ast.StarExpr); ok {
		return exprName(s.X)
	}
	return exprName(t)
}

func exprName(t ast.Expr) string {
	switch n := t.(type) {
	case *ast.Ident:
		return n.Name
	case *ast.StarExpr:
		return exprName(n.X)
	case *ast.SelectorExpr:
		return n.Sel.Name
	}
	return "?"
}

func sortedNames(m map[string][]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
