package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// 코드를 쓰기 전에 **이 저장소에 실제로 있는 이름**을 캐서 보여 준다.
//
// W-19079·W-33064 가 없는 이름을 불러 막혔다.
//
//	getConnectClient().updateInviteStatus(...)   ← 그런 RPC 는 없다
//	cabinet.updateConnectionStatus              ← 그런 내보내기는 없다
//	LaborContract.isPending                     ← 그런 필드는 없다
//
// 사람은 이럴 때 grep 한 번 한다. 있는 이름을 보고 나서 쓴다. 기계는 그
// 한 걸음을 건너뛰고 그럴듯한 이름을 지어냈다.
//
// 그래서 고칠 파일마다, 그 파일이 들여오는 **저장소 안 모듈의 내보낸
// 이름**과, 그 모듈에서 온 공장 함수에 **실제로 쓰인 메서드**를 모아
// 코더에게 먼저 준다. 색인도 모델도 아니고 파일을 읽어 센 것이라 틀릴 수가
// 없다.

var (
	reImport      = regexp.MustCompile(`(?m)^\s*import\s+(?:type\s+)?(?:\{([^}]*)\}|(\w+))\s+from\s+["']([^"']+)["']`)
	reExportNamed = regexp.MustCompile(`(?m)^\s*export\s+(?:declare\s+)?(?:async\s+)?(?:function|const|let|var|class|interface|type|enum)\s+(\w+)`)
	reExportList  = regexp.MustCompile(`(?m)^\s*export\s*\{([^}]*)\}`)
	reGoExport    = regexp.MustCompile(`(?m)^(?:func|type|const|var)\s+\(?[^)]*\)?\s*([A-Z]\w*)`)
)

// tsModulePath 는 들여오기 지정자를 저장소 안 파일로 푼다. 밖이면 빈 문자열이다.
func tsModulePath(repoPath, fromFile, spec string) string {
	var base string
	switch {
	case strings.HasPrefix(spec, "$lib/"):
		base = filepath.Join(repoPath, "src", "lib", strings.TrimPrefix(spec, "$lib/"))
	case spec == "$lib":
		base = filepath.Join(repoPath, "src", "lib")
	case strings.HasPrefix(spec, "$"):
		return "" // $app 등 프레임워크 것
	case strings.HasPrefix(spec, "."):
		base = filepath.Join(filepath.Dir(filepath.Join(repoPath, fromFile)), spec)
	default:
		return "" // node_modules
	}
	for _, cand := range []string{base, base + ".ts", base + ".js", base + ".svelte",
		filepath.Join(base, "index.ts"), filepath.Join(base, "index.js")} {
		if st, err := os.Stat(cand); err == nil && !st.IsDir() {
			return cand
		}
	}
	return ""
}

// exportedNames 는 그 파일이 내보내는 이름을 센다.
func exportedNames(path string) []string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	src := string(b)
	seen := map[string]bool{}
	for _, m := range reExportNamed.FindAllStringSubmatch(src, -1) {
		seen[m[1]] = true
	}
	for _, m := range reExportList.FindAllStringSubmatch(src, -1) {
		for _, part := range strings.Split(m[1], ",") {
			part = strings.TrimSpace(part)
			if i := strings.Index(part, " as "); i >= 0 {
				part = strings.TrimSpace(part[i+4:])
			}
			if part != "" && !strings.ContainsAny(part, "{}*") {
				seen[part] = true
			}
		}
	}
	return sortedKeys(seen)
}

// calledMethods 는 저장소 전체에서 `name()` 뒤에 실제로 쓰인 메서드를 센다.
//
// 이것이 핵심이다. 내보내기 목록만으로는 gRPC 클라이언트에 어떤 RPC 가
// 있는지 알 수 없다 — 타입은 생성물 안에 있다. 그런데 **이미 쓰이고 있는
// 호출**은 저장소에 그대로 적혀 있다. 사람이 grep 으로 보는 그것이다.
func calledMethods(repoPath, factory string) []string {
	re := regexp.MustCompile(regexp.QuoteMeta(factory) + `\(\)\s*\.\s*(\w+)`)
	seen := map[string]bool{}
	_ = filepath.WalkDir(filepath.Join(repoPath, "src"), func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		switch filepath.Ext(p) {
		case ".ts", ".js", ".svelte":
		default:
			return nil
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return nil
		}
		for _, m := range re.FindAllStringSubmatch(string(b), -1) {
			seen[m[1]] = true
		}
		return nil
	})
	return sortedKeys(seen)
}

// AvailableNames 는 이 파일을 고치는 코더에게 줄 "있는 이름" 쪽지다.
// 캘 것이 없으면 빈 문자열 — 그때는 아무것도 붙이지 않는다.
func AvailableNames(repoPath, file string) string {
	if filepath.Ext(file) == ".go" {
		return availableNamesGo(repoPath, file)
	}
	b, err := os.ReadFile(filepath.Join(repoPath, file))
	if err != nil {
		return ""
	}
	src := string(b)

	type mod struct {
		spec    string
		exports []string
		comp    string   // svelte 면 기본 내보내기 이름
		props   []string // svelte 면 넘길 수 있는 속성
	}
	var mods []mod
	var factories []string

	seenSpec := map[string]bool{}
	for _, m := range reImport.FindAllStringSubmatch(src, -1) {
		spec := m[3]
		if seenSpec[spec] {
			continue
		}
		seenSpec[spec] = true
		p := tsModulePath(repoPath, file, spec)
		if p == "" {
			continue
		}
		if strings.HasSuffix(p, ".svelte") {
			mods = append(mods, mod{spec: spec, comp: componentName(p), props: svelteProps(p)})
			if len(mods) >= 12 {
				break
			}
			continue
		}
		ex := exportedNames(p)
		if len(ex) == 0 {
			continue
		}
		mods = append(mods, mod{spec: spec, exports: relevantFirst(ex, src)})

		// 들여온 이름 가운데 이 파일에서 `X()` 꼴로 쓰이는 것은 공장이다.
		names := m[1]
		if names == "" {
			names = m[2]
		}
		for _, n := range strings.Split(names, ",") {
			n = strings.TrimSpace(n)
			if i := strings.Index(n, " as "); i >= 0 {
				n = strings.TrimSpace(n[i+4:])
			}
			if n == "" || !strings.Contains(src, n+"()") {
				continue
			}
			factories = append(factories, n)
		}
		if len(mods) >= 12 {
			break
		}
	}

	if len(mods) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("[이 저장소에 실제로 있는 이름 — 파일을 읽어 센 것이다. 여기 없는 이름을 쓰면 빌드가 깨진다]\n")
	for _, m := range mods {
		if m.comp != "" {
			sb.WriteString("  " + m.spec + " → 기본 내보내기 " + m.comp +
				" 하나뿐이다 (import " + m.comp + " from '" + m.spec + "')\n")
			if len(m.props) > 0 {
				sb.WriteString("      넘길 수 있는 속성(가져오는 이름이 아니다): " +
					strings.Join(clipNames(m.props, 20), ", ") + "\n")
			}
			continue
		}
		sb.WriteString("  " + m.spec + " → " + strings.Join(clipNames(m.exports, 40), ", ") + "\n")
	}
	for _, f := range factories {
		if ms := calledMethods(repoPath, f); len(ms) > 0 {
			sb.WriteString(fmt.Sprintf("  %s() 에 이 저장소가 실제로 쓰는 것: %s\n",
				f, strings.Join(clipNames(ms, 60), ", ")))
		}
	}
	var cand []string
	for _, m := range mods {
		cand = append(cand, m.exports...)
	}
	if fields := typeFieldSheetFor(repoPath, cand, 6); fields != "" {
		sb.WriteString(fields)
	}
	sb.WriteString("없는 이름이 필요하면 지어내지 마라 — 무엇이 없어서 못 했는지 적어라.\n\n")
	return sb.String()
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func clipNames(xs []string, n int) []string {
	if len(xs) <= n {
		return xs
	}
	return append(xs[:n:n], "…")
}

// **Svelte 파일의 `export let` 은 속성이지 가져올 이름이 아니다.**
//
// 이 쪽지가 그것을 "있는 이름" 으로 적었고, 코더가 그대로 가져왔다.
//
//	import { description, failed, title } from '$lib/components/web/EmptyState.svelte';
//	import { description } from '$lib/components/web/SectionTitle.svelte';
//	→ Identifier 'description' has already been declared
//
// 컴포넌트에서 가져올 수 있는 것은 **기본 내보내기 하나**, 곧 컴포넌트
// 자신뿐이다. 속성은 넘기는 것이지 가져오는 것이 아니다. 둘을 갈라 적는다.
// 이름은 파일 이름 그대로 쓴다 — pageheader 로 적어 파일을 못 찾은 적이 있다.
var reSvelteProp = regexp.MustCompile(`(?m)^\s*export\s+let\s+(\w+)`)

func svelteProps(path string) []string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	for _, m := range reSvelteProp.FindAllStringSubmatch(string(b), -1) {
		seen[m[1]] = true
	}
	return sortedKeys(seen)
}

// componentName 은 파일 이름 그대로의 컴포넌트 이름이다.
func componentName(path string) string {
	return strings.TrimSuffix(filepath.Base(path), ".svelte")
}
