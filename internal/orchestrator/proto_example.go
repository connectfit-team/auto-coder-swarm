package orchestrator

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var protoMessageRe = regexp.MustCompile(`(?m)^message\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{`)

// protoConventionExample 은 같은 계약 파일에 이미 있는 Request/Response 짝을
// 그대로 보여 준다.
//
// 이름 규칙만 일러 주는 것으로는 모자랐다. 실측(W-99311)에서 접두형은 맞게
// 썼는데 응답 모양을 지어냈다 — 이 계약의 응답은 모두
// `ResponseAcceptRequest { bool accepted = 1; }` 처럼 동작 이름을 딴 불린
// 하나인데, `bool success` + `string error_message` 를 냈다. 그 두 이름은
// 이 계약 어디에도 없다(오류는 gRPC 상태로 간다).
//
// 사람은 이럴 때 옆에 있는 짝을 한 번 보고 쓴다. 그 한 걸음을 준다.
func protoConventionExample(repoPath, file string, maxPairs int) string {
	if filepath.Ext(file) != ".proto" || maxPairs <= 0 {
		return ""
	}
	b, err := os.ReadFile(filepath.Join(repoPath, file))
	if err != nil {
		return ""
	}
	src := string(b)
	blocks := protoBlocks(src)

	var out []string
	for name, body := range blocks {
		if len(out) >= maxPairs || !strings.HasPrefix(name, "Request") {
			continue
		}
		resp, ok := blocks["Response"+strings.TrimPrefix(name, "Request")]
		if !ok {
			continue
		}
		// 짧은 짝이 본보기로 낫다 — 길면 프롬프트만 먹고 규칙은 안 보인다.
		if strings.Count(body, "\n")+strings.Count(resp, "\n") > 16 {
			continue
		}
		out = append(out, body+"\n"+resp)
	}
	if len(out) == 0 {
		return ""
	}
	return "[이 계약이 요청·응답을 쓰는 꼴 — 그대로 본떠라]\n" +
		strings.Join(out, "\n") +
		"\n응답은 이 꼴을 벗어나지 마라. 이 계약에 없는 필드 이름을 지어내지 마라.\n\n"
}

// protoBlocks 는 message 이름 → 그 블록 전문을 준다.
func protoBlocks(src string) map[string]string {
	blocks := map[string]string{}
	for _, m := range protoMessageRe.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		depth, end := 0, -1
		for i := m[1] - 1; i < len(src); i++ {
			switch src[i] {
			case '{':
				depth++
			case '}':
				if depth--; depth == 0 {
					end = i + 1
				}
			}
			if end >= 0 {
				break
			}
		}
		if end > m[0] {
			blocks[name] = src[m[0]:end]
		}
	}
	return blocks
}
