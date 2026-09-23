package agent

import (
	"regexp"
	"strings"
)

// 통짜로 다시 쓰다가 **내주던 이름을 잃는 것**을 막는다.
//
// 큰 파일을 통째로 다시 내게 하면 모델이 뒷부분을 조용히 빠뜨린다. 빌드가
// 통과해도 부르던 쪽이 깨진다 — 그 파일이 밖으로 내주던 이름이 사라졌기
// 때문이다. 고치기 전후의 이름을 세어 견준다.

// 파일이 밖으로 내주던 이름. 이것이 사라지면 부르던 쪽이 깨진다.
var exportPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?m)^\s*export\s+(?:default\s+)?(?:async\s+)?function\s+([A-Za-z_$][\w$]*)`),
	regexp.MustCompile(`(?m)^\s*export\s+(?:const|let|var|class|interface|type|enum)\s+([A-Za-z_$][\w$]*)`),
	regexp.MustCompile(`(?m)^\s*func\s+([A-Z][\w]*)\s*\(`),
}

// exportedNames 는 파일이 밖으로 내주는 이름을 모은다.
// 계약(.proto)은 선언 모양이 다르다. export 가 없으니 위 패턴으로는 아무것도
// 안 잡히고, 통째로 다시 쓰다가 메시지를 통째로 잃어도 아무도 모른다.
var protoDeclPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?m)^\s*(?:message|enum|service)\s+([A-Za-z_]\w*)`),
	regexp.MustCompile(`(?m)^\s*rpc\s+([A-Za-z_]\w*)\s*\(`),
}

// exportedNamesFor 는 그 말에 맞는 선언 이름을 모은다.
func exportedNamesFor(path, src string) map[string]bool {
	if strings.HasSuffix(path, ".proto") {
		out := map[string]bool{}
		for _, re := range protoDeclPatterns {
			for _, m := range re.FindAllStringSubmatch(src, -1) {
				out[m[1]] = true
			}
		}
		return out
	}
	return exportedNames(src)
}

func exportedNames(src string) map[string]bool {
	out := map[string]bool{}
	for _, re := range exportPatterns {
		for _, m := range re.FindAllStringSubmatch(src, -1) {
			out[m[1]] = true
		}
	}
	// export { a, b as c }
	braceRe := regexp.MustCompile(`(?s)export\s*\{([^}]*)\}`)
	for _, m := range braceRe.FindAllStringSubmatch(src, -1) {
		for _, part := range strings.Split(m[1], ",") {
			part = strings.TrimSpace(part)
			if i := strings.LastIndex(strings.ToLower(part), " as "); i >= 0 {
				part = strings.TrimSpace(part[i+4:])
			}
			if part != "" {
				out[part] = true
			}
		}
	}
	return out
}

// lostExports 는 고친 뒤 사라진 이름을 준다.
//
// 통짜로 다시 쓰게 하면 모델이 조용히 함수를 빠뜨린다. 빌드가 깨지고 나서야
// 알게 되는데, 그때는 이미 자가 치유 세 번을 태운 뒤다.
func lostExports(before, after string) []string {
	return lostExportsFor("", before, after)
}

// lostExportsFor 는 그 말에 맞는 선언으로 견준다.
func lostExportsFor(path, before, after string) []string {
	had := exportedNamesFor(path, before)
	has := exportedNamesFor(path, after)
	var lost []string
	for name := range had {
		if !has[name] {
			lost = append(lost, name)
		}
	}
	return lost
}
