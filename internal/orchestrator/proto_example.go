package orchestrator

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	protoMessageRe = regexp.MustCompile(`(?m)^message\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{`)
	protoEnumRe    = regexp.MustCompile(`(?m)^enum\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{`)
	protoValueRe   = regexp.MustCompile(`^\s*([A-Z][A-Z0-9_]*)\s*=\s*(\d+)\s*;`)
	protoServiceRe = regexp.MustCompile(`(?m)^service\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{`)
	protoRPCRe     = regexp.MustCompile(`^\s*(rpc\s+.+)$`)
)

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
		"\n응답은 이 꼴을 벗어나지 마라. 이 계약에 없는 필드 이름을 지어내지 마라.\n\n" +
		protoEnumExample(repoPath, file) +
		protoServiceExample(repoPath, file)
}

// protoServiceExample 은 이 파일에 이미 있는 service 를 짧게 보여 준다.
//
// 관문이 가장 자주 문 것이다 — "service ConnectService 를 새로 만들었다,
// 이 파일에는 이미 service Internal 가 있다" (실측 W-33314·W-24124, 둘 다
// 시도를 다 쓰고 죽었다). 새 service 는 쓰는 쪽이 부르지 않으므로 조용히
// 아무 일도 일어나지 않는다.
func protoServiceExample(repoPath, file string) string {
	b, err := os.ReadFile(filepath.Join(repoPath, file))
	if err != nil {
		return ""
	}
	src := string(b)
	m := protoServiceRe.FindStringSubmatchIndex(src)
	if m == nil {
		return ""
	}
	name := src[m[2]:m[3]]
	var sb strings.Builder
	sb.WriteString("service " + name + " {\n")
	// rpc 는 `{}` 본문을 가질 수 있다. 맨 앞의 `}` 로 끊으면 그 본문의 닫는
	// 괄호에서 멈춰 rpc 를 하나밖에 못 보여 준다.
	kept, depth := 0, 1
	for _, line := range strings.Split(src[m[1]:], "\n") {
		if r := protoRPCRe.FindStringSubmatch(line); r != nil && depth == 1 {
			one := strings.TrimSpace(r[1])
			one = strings.TrimSuffix(strings.TrimSpace(strings.TrimSuffix(one, "{")), "{")
			sb.WriteString("  " + strings.TrimSpace(one) + "\n")
			kept++
		}
		depth += strings.Count(line, "{") - strings.Count(line, "}")
		if depth <= 0 || kept >= 2 {
			break
		}
	}
	if kept == 0 {
		return ""
	}
	sb.WriteString("  ...\n}\n")
	return "[이 파일에는 이미 service " + name + " 가 있다 — **그 안에** rpc 를 더해라]\n" +
		sb.String() +
		"새 service 를 만들면 쓰는 쪽이 부르지 않아 조용히 아무 일도 일어나지 않는다.\n\n"
}

// protoEnumExample 은 이 계약에 이미 있는 enum 을 짧게 보여 준다.
//
// 관문이 되풀이해 문 것이 0 번이었다 — "enum RequestStatus 의 0 번이 PENDING
// 다" (실측 W-24124). 0 은 값이 없을 때의 기본값이라, 여태 있던 데이터가 모두
// 그 뜻으로 읽힌다. 규칙으로 일러 주는 것보다 이 계약의 enum 을 한 번 보여
// 주는 편이 낫다.
func protoEnumExample(repoPath, file string) string {
	dir := filepath.Dir(filepath.Join(repoPath, file))
	paths := []string{filepath.Join(repoPath, file)}
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && filepath.Ext(e.Name()) == ".proto" {
				paths = append(paths, filepath.Join(dir, e.Name()))
			}
		}
	}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if shown := firstEnumDigest(string(b)); shown != "" {
			return "[이 계약이 enum 을 쓰는 꼴 — 0 번은 반드시 「정해지지 않음」 이다]\n" +
				shown +
				"0 번에 뜻을 넣으면 여태 있던 것이 모두 그 값으로 읽힌다.\n\n"
		}
	}
	return ""
}

// firstEnumDigest 는 첫 enum 의 머리와 앞 세 값만 남긴다. 긴 것을 통째로
// 넣으면 프롬프트만 먹고 정작 규칙은 안 보인다.
func firstEnumDigest(src string) string {
	m := protoEnumRe.FindStringSubmatchIndex(src)
	if m == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(src[m[0]:m[1]] + "\n")
	kept := 0
	for _, line := range strings.Split(src[m[1]:], "\n") {
		if strings.TrimSpace(line) == "}" {
			break
		}
		if v := protoValueRe.FindStringSubmatch(line); v != nil {
			b.WriteString("  " + v[1] + " = " + v[2] + ";\n")
			if kept++; kept >= 3 {
				break
			}
		}
	}
	if kept == 0 {
		return ""
	}
	b.WriteString("  ...\n}\n")
	return b.String()
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
