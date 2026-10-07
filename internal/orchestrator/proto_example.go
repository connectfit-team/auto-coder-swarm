package orchestrator

import (
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
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
	src := protoAtHead(repoPath, file)
	if src == "" {
		return ""
	}

	out := requestPairsIn(protoBlocks(src), maxPairs)
	// **짝이 그 파일에 없으면 이웃 파일에서 가져온다.**
	//
	// 관행은 파일이 아니라 꾸러미의 것이다. 이 계약의 service 파일에는
	// 메시지가 하나도 없고(`service Internal` 뿐이다) 요청·응답은 전부
	// communication 파일에 있다. 그런데 새 Request/Response 를 만드는 자리는
	// 바로 그 service 파일이다 — 본보기가 가장 필요한 곳에 본보기가 없었다.
	if len(out) == 0 {
		out = requestPairsNearby(repoPath, file, maxPairs)
	}

	var b strings.Builder
	if len(out) > 0 {
		b.WriteString("[이 계약이 요청·응답을 쓰는 꼴 — 그대로 본떠라]\n")
		b.WriteString(strings.Join(out, "\n"))
		b.WriteString("\n응답은 이 꼴을 벗어나지 마라. 이 계약에 없는 필드 이름을 지어내지 마라.\n\n")
	}
	// **짝이 없다고 나머지까지 버리지 않는다.**
	//
	// 전에는 짝이 없으면 통째로 빈 문자열을 냈다. 그래서 service 파일을 고치는
	// 코더가 상태 타입 예시(#170)도, service 예시(#152)도, enum 예시도 **하나도
	// 못 받았다.** 실측 W-83289 가 `string status`, W-16789 가 `bool hold_status`
	// 를 낸 자리가 정확히 거기다. 셋은 짝과 아무 상관이 없다.
	b.WriteString(protoStateFieldExample(repoPath))
	b.WriteString(protoEnumExample(repoPath, file))
	b.WriteString(protoServiceExample(repoPath, file))
	return b.String()
}

// requestPairsIn 은 한 파일 안의 짧은 Request/Response 짝을 고른다.
func requestPairsIn(blocks map[string]string, maxPairs int) []string {
	var names []string
	for name := range blocks {
		if strings.HasPrefix(name, "Request") {
			names = append(names, name)
		}
	}
	sort.Strings(names) // 회차마다 같은 글이 되게

	var out []string
	for _, name := range names {
		if len(out) >= maxPairs {
			break
		}
		body := blocks[name]
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
	return out
}

// requestPairsNearby 는 같은 폴더(= 같은 꾸러미)의 이웃 파일에서 짝을 찾는다.
func requestPairsNearby(repoPath, file string, maxPairs int) []string {
	dir := path.Dir(filepath.ToSlash(file))
	var rels []string
	forEachProto(repoPath, func(rel, _ string) {
		if rel != filepath.ToSlash(file) && path.Dir(rel) == dir {
			rels = append(rels, rel)
		}
	})
	sort.Strings(rels)
	var out []string
	for _, rel := range rels {
		if len(out) >= maxPairs {
			break
		}
		src := protoAtHead(repoPath, rel)
		if src == "" {
			continue
		}
		out = append(out, requestPairsIn(protoBlocks(src), maxPairs-len(out))...)
	}
	return out
}

// protoServiceExample 은 이 파일에 이미 있는 service 를 짧게 보여 준다.
//
// 관문이 가장 자주 문 것이다 — "service ConnectService 를 새로 만들었다,
// 이 파일에는 이미 service Internal 가 있다" (실측 W-33314·W-24124, 둘 다
// 시도를 다 쓰고 죽었다). 새 service 는 쓰는 쪽이 부르지 않으므로 조용히
// 아무 일도 일어나지 않는다.
func protoServiceExample(repoPath, file string) string {
	src := protoAtHead(repoPath, file)
	if src == "" {
		return ""
	}
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
		rel, err := filepath.Rel(repoPath, p)
		if err != nil {
			continue
		}
		if shown := firstEnumDigest(protoAtHead(repoPath, rel)); shown != "" {
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

// protoAtHead 는 **고치기 전**의 그 파일을 준다.
//
// 작업 트리에서 읽으면 안 된다. 관문에 막혀 다시 시킬 때 트리에는 앞 시도의
// 수정이 그대로 남아 있으므로(고친 것을 버리지 않는다), 이웃 예시로 **모델이
// 제 실수를 내밀게 된다** — 앞 시도에서 잘못 만든 메시지를 이 계약의 관행인
// 양 본떠 쓰게 된다.
//
// HEAD 를 못 읽으면 트리에서 읽는다. 예시가 아예 없는 것보다는 낫다.
func protoAtHead(repoPath, rel string) string {
	out, err := exec.Command("git", "-C", repoPath, "show", "HEAD:"+rel).Output()
	if err == nil && len(out) > 0 {
		return string(out)
	}
	b, err := os.ReadFile(filepath.Join(repoPath, rel))
	if err != nil {
		return ""
	}
	return string(b)
}
