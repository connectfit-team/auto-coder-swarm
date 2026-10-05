package orchestrator

import (
	"fmt"
	"regexp"
	"strings"
)

var reStateDecl = regexp.MustCompile(`^\s*((?:repeated\s+|optional\s+)?[\w.]+)\s+(\w*(?:status|state))\s*=\s*(\d+)\s*;(.*)$`)

// protoStateFieldExample 은 이 계약에 **이미 있는 상태 필드**를 그대로 보여 준다.
//
// 관문이 `string status` 를 막고 "이 계약이 실제로 쓰는 것: int32(3곳)" 이라고
// 일러 준다. 그런데 측정에서 모델은 **세 시도 내내 다시 string 을 냈다**
// (W-70015·W-96305·W-62264). 말로 이르는 것과 이웃을 한 번 보여 주는 것은
// 다르다 — 응답 모양(#151)도, 새 service·enum 0 번(#152)도 보여 주고서야
// 멈췄다.
//
// 그래서 세어서 말하는 대신 **진짜 줄을 보여 준다.** 주석까지 함께 — 이 계약은
// int32 에 뜻을 주석으로 적어 두는 꼴이라, 그것을 봐야 따라 쓸 수 있다.
func protoStateFieldExample(repoPath string) string {
	type decl struct{ line, from string }
	var found []decl
	seen := map[string]bool{}

	forEachProto(repoPath, func(rel, _ string) {
		src := protoAtHead(repoPath, rel)
		if src == "" {
			return
		}
		for _, line := range strings.Split(src, "\n") {
			m := reStateDecl.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			text := strings.TrimSpace(m[1] + " " + m[2] + " = " + m[3] + ";" + m[4])
			if seen[text] {
				continue
			}
			seen[text] = true
			found = append(found, decl{text, rel})
		}
	})
	if len(found) == 0 {
		return ""
	}
	if len(found) > 4 {
		found = found[:4]
	}

	var b strings.Builder
	b.WriteString("[이 계약이 상태를 담는 꼴 — 그대로 따라라]\n")
	for _, d := range found {
		fmt.Fprintf(&b, "  %s\n      (%s)\n", d.line, d.from)
	}
	b.WriteString("상태를 string 으로 담지 마라. 글자는 굳지 않아서 부르는 쪽마다 다른 것을 넣는다.\n" +
		"위 꼴을 쓰거나, 더 나은 것을 원하면 enum 을 만들어라 — enum 은 막지 않는다.\n\n")
	return b.String()
}
