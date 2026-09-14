package orchestrator

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Go 저장소에서도 **있는 이름**을 먼저 보여 준다.
//
// 쪽지가 TypeScript 만 보고 있었다. 그런데 이 회사 저장소는 대부분 Go 다 —
// 없는 이름을 부르는 실수가 거기서 안 날 이유가 없다.
//
// 파서로 읽는다. 정규식으로 export 를 긁는 것보다 정확하고, 표준 라이브러리에
// 이미 있는 것을 쓰지 않을 까닭이 없다.

var reGoModule = regexp.MustCompile(`(?m)^module\s+(\S+)`)

// goModulePath 는 go.mod 의 모듈 경로다. 없으면 빈 문자열이다.
func goModulePath(repoPath string) string {
	b, err := os.ReadFile(filepath.Join(repoPath, "go.mod"))
	if err != nil {
		return ""
	}
	if m := reGoModule.FindSubmatch(b); m != nil {
		return string(m[1])
	}
	return ""
}

// goExported 는 그 꾸러미가 내보내는 이름을 준다(시험 파일은 뺀다).
func goExported(dir string) []string {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	for _, pkg := range pkgs {
		for _, f := range pkg.Files {
			for _, d := range f.Decls {
				switch t := d.(type) {
				case *ast.FuncDecl:
					if t.Name.IsExported() {
						name := t.Name.Name
						// 메서드는 받는 타입과 함께 적어야 쓸모가 있다.
						if t.Recv != nil && len(t.Recv.List) > 0 {
							name = recvName(t.Recv.List[0].Type) + "." + name
						}
						seen[name] = true
					}
				case *ast.GenDecl:
					for _, sp := range t.Specs {
						switch s := sp.(type) {
						case *ast.TypeSpec:
							if s.Name.IsExported() {
								seen[s.Name.Name] = true
							}
						case *ast.ValueSpec:
							for _, n := range s.Names {
								if n.IsExported() {
									seen[n.Name] = true
								}
							}
						}
					}
				}
			}
		}
	}
	return sortedKeys(seen)
}

func recvName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return recvName(t.X)
	case *ast.Ident:
		return t.Name
	}
	return "?"
}

// availableNamesGo 는 이 Go 파일이 들여오는 **저장소 안** 꾸러미의 이름을 준다.
func availableNamesGo(repoPath, file string) string {
	mod := goModulePath(repoPath)
	if mod == "" {
		return ""
	}
	full := filepath.Join(repoPath, file)
	src, _ := os.ReadFile(full)
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, full, nil, parser.ImportsOnly)
	if err != nil {
		return ""
	}

	type entry struct {
		imp   string
		names []string
	}
	var got []entry
	for _, im := range f.Imports {
		path := strings.Trim(im.Path.Value, `"`)
		if !strings.HasPrefix(path, mod+"/") {
			continue // 밖의 꾸러미는 안 본다
		}
		dir := filepath.Join(repoPath, strings.TrimPrefix(path, mod+"/"))
		if names := goExported(dir); len(names) > 0 {
			got = append(got, entry{path, relevantFirst(names, string(src))})
		}
		if len(got) >= 10 {
			break
		}
	}
	if len(got) == 0 {
		return ""
	}

	sort.Slice(got, func(i, j int) bool { return got[i].imp < got[j].imp })
	var b strings.Builder
	b.WriteString("[이 저장소에 실제로 있는 이름 — 파서로 읽은 것이다. 여기 없는 이름을 쓰면 빌드가 깨진다]\n")
	for _, e := range got {
		b.WriteString("  " + e.imp + " → " + strings.Join(clipNames(e.names, 50), ", ") + "\n")
	}
	b.WriteString("없는 이름이 필요하면 지어내지 마라 — 무엇이 없어서 못 했는지 적어라.\n\n")
	return b.String()
}

// relevantFirst 는 **이 파일이 쓰는 말과 가까운 이름부터** 보이게 한다.
//
// 알파벳순으로 자르면 A 로 시작하는 것만 남는다 — 한 저장소에서 실제로
// 재 보니 도메인 이름 50개가 전부 Attendance… 로 시작했고, 정작 고치려는
// 자리의 이름은 잘려 나갔다. 쪽지는 **짧아야** 읽히므로 고르는 것이 중요하다.
//
// 고르는 법: 이미 이 파일에 나오는 이름이 먼저다. 그다음은 파일의 낱말과
// 겹치는 것, 나머지는 알파벳순.
func relevantFirst(names []string, hint string) []string {
	low := strings.ToLower(hint)
	words := map[string]bool{}
	for _, w := range regexp.MustCompile(`[A-Za-z]{4,}`).FindAllString(low, -1) {
		words[w] = true
	}

	score := func(n string) int {
		base := n
		if i := strings.LastIndex(n, "."); i >= 0 {
			base = n[i+1:]
		}
		if strings.Contains(hint, base) {
			return 2 // 이 파일이 이미 쓰고 있다
		}
		for _, part := range splitCamel(base) {
			if len(part) >= 4 && words[part] {
				return 1 // 말이 겹친다
			}
		}
		return 0
	}

	out := append([]string(nil), names...)
	sort.SliceStable(out, func(i, j int) bool {
		si, sj := score(out[i]), score(out[j])
		if si != sj {
			return si > sj
		}
		return out[i] < out[j]
	})
	return out
}

// splitCamel 은 AttendanceSetup 을 attendance·setup 으로 쪼갠다.
func splitCamel(s string) []string {
	var out []string
	start := 0
	for i := 1; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' {
			out = append(out, strings.ToLower(s[start:i]))
			start = i
		}
	}
	return append(out, strings.ToLower(s[start:]))
}
