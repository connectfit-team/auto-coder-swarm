package orchestrator

import (
	"strings"
)

// protoGuides 는 계약 파일을 고칠 때 들어 있어야 할 본보기들이다.
//
// 글머리로 찾는다. 글을 고치면 여기도 고쳐야 하지만, 그 편이 「있다고 쳤는데
// 없었다」 보다 낫다 — 짝을 못 찾으면 셋이 함께 사라지던 일을 이걸로 잡는다.
var protoGuides = []struct{ mark, name string }{
	{"[이 계약이 상태를 담는 꼴", "상태 타입"},
	{"[이 파일에는 이미 service", "이미 있는 service"},
	{"[이 계약이 enum 을 쓰는 꼴", "enum 꼴"},
}

// missingProtoGuides 는 계약 파일 안내문에서 빠진 본보기를 말한다.
//
// 모든 파일에 셋이 다 맞는 것은 아니다 — service 가 없는 파일에 service 예시는
// 못 만든다. 그래서 **하나도 없을 때만** 말한다. 그때는 틀림없이 무언가 끊긴
// 것이다(실측: 안내문 44자).
func missingProtoGuides(file, instr string) string {
	if !strings.HasSuffix(file, ".proto") {
		return ""
	}
	var missing []string
	have := 0
	for _, g := range protoGuides {
		if strings.Contains(instr, g.mark) {
			have++
		} else {
			missing = append(missing, g.name)
		}
	}
	if have > 0 {
		return ""
	}
	return strings.Join(missing, " · ") + " 가 모두 빠졌다"
}
