package agent

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// 같은 이름을 두 번 선언하는 것을 쓰기 전에 막는다.
//
// 한 파일에 `let pending` 이 두 번 들어가 빌드가 깨졌다(W-71948).
//
//	src/routes/.../+page.svelte (48:3): Identifier "pending" has already been declared
//
// 고쳐 쓰는 자리에 이미 있는 이름을 다시 선언한 것이다. 치유기가 같은 것을
// 또 넣기도 한다 — 그러면 회차마다 같은 오류가 나고 두 번 만에 멈춘다.
//
// **겉껍질(중괄호 깊이 0)에서만 본다.** 함수 안쪽의 같은 이름은 서로 다른
// 유효범위라 잘못이 아니다. 그것까지 세면 멀쩡한 코드를 막는다.

// `\w` 는 ASCII 만 본다. 이 회사 Go 시험 이름에는 한글이 들어간다 —
// `TestDenyCMSReadOnly_거절한다` 가 `TestDenyCMSReadOnly_` 로 잘려서 서로
// 다른 시험 다섯이 같은 이름으로 보였다(실측: 세 저장소에서 71건 오탐).
const identPat = `[\p{L}\p{N}_]+`

var reDeclJS = regexp.MustCompile(`^\s*(?:export\s+)?(?:async\s+)?(let|const|var|function|class)\s+(` + identPat + `)`)
var reDeclGo = regexp.MustCompile(`^(func|type)\s+(` + identPat + `)`)

// duplicateDecls 는 겉껍질에서 두 번 이상 선언된 이름을 준다.
func duplicateDecls(path, content string) []string {
	ext := filepath.Ext(path)
	if ext == ".proto" {
		return duplicateProtoDecls(content)
	}
	js := ext == ".ts" || ext == ".js" || ext == ".mjs" || ext == ".svelte" || ext == ".tsx"
	if !js && ext != ".go" {
		return nil
	}

	seen := map[string]int{}
	depth := 0
	inScript := ext != ".svelte" // .svelte 는 <script> 안에서만 본다
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if ext == ".svelte" {
			if strings.HasPrefix(trimmed, "<script") {
				inScript, depth = true, 0
				continue
			}
			if strings.HasPrefix(trimmed, "</script") {
				inScript = false
				continue
			}
		}
		if inScript && depth == 0 {
			var m []string
			if js {
				m = reDeclJS.FindStringSubmatch(line)
			} else {
				m = reDeclGo.FindStringSubmatch(line)
			}
			if m != nil {
				seen[m[2]]++
			}
		}
		if inScript {
			depth += strings.Count(line, "{") - strings.Count(line, "}")
			if depth < 0 {
				depth = 0
			}
		}
	}

	var dup []string
	for name, n := range seen {
		if n > 1 {
			dup = append(dup, fmt.Sprintf("%s (%d번)", name, n))
		}
	}
	return dup
}

// duplicateProtoDecls 는 한 계약 파일 안에서 같은 이름을 두 번 선언했는지 본다.
//
// 통째로 다시 쓰게 하면 모델이 있는 메시지를 지우지 않은 채 같은 이름으로
// 하나 더 만든다. 빌드는 계약을 안 보므로 아무도 못 막는다 — 쓰기 전에 본다.
func duplicateProtoDecls(content string) []string {
	re := regexp.MustCompile(`(?m)^\s*(?:message|enum|service)\s+([A-Za-z_]\w*)`)
	seen := map[string]int{}
	var order []string
	for _, m := range re.FindAllStringSubmatch(content, -1) {
		if seen[m[1]] == 0 {
			order = append(order, m[1])
		}
		seen[m[1]]++
	}
	var out []string
	for _, name := range order {
		if seen[name] > 1 {
			out = append(out, name)
		}
	}
	return out
}
