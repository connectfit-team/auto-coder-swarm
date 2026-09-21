package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// 손으로 쓴 타입이어도 **그 데이터가 어디서 오는지**는 파일에 적혀 있다.
//
// 「연결 요청에 보류 상태를 담을 필드」 가 없다고 했고, 모델은 그것이
// ReceivedRequest 에 붙어야 한다고 바르게 답했다. 그런데 그 타입은 이 저장소가
// 손으로 쓴 것이라 거기서 멈췄다(W-77123).
//
// 멈출 자리가 아니다. 그 타입을 채우는 것은 RPC 이고, 고리가 전부 적혀 있다.
//
//	ReceivedRequest              ← connectcud.ts 가 쓴 타입
//	  getConnectClient()         ← 그 파일이 부르는 공장
//	  ConnectCEOWebDefinition    ← clients.ts 의 그 공장 정의가 쓰는 이름
//	  …/protos/ceowebapis/…      ← 그 이름을 들여오는 경로
//	  proto-ceowebapis           ← 임자
//
// 한 걸음도 모델에게 묻지 않는다. 파일을 읽어 따라간다.

var (
	reClientCall = regexp.MustCompile(`\b(get[\p{L}\p{N}_]*Client)\s*\(\s*\)`)
	reImportFrom = regexp.MustCompile(`(?s)import\s+(?:type\s+)?\{([^}]*)\}\s+from\s+["']([^"']+)["']`)
)

// protoOwnerViaClient 는 그 파일이 부르는 gRPC 공장을 따라가 임자 저장소를 찾는다.
func protoOwnerViaClient(repoPath, file string) (string, string) {
	b, err := os.ReadFile(filepath.Join(repoPath, file))
	if err != nil {
		return "", ""
	}
	src := string(b)

	m := reClientCall.FindStringSubmatch(src)
	if m == nil {
		return "", "이 파일은 gRPC 공장을 부르지 않는다"
	}
	factory := m[1]

	// 그 공장이 어느 모듈에서 오는지
	var clientsMod string
	for _, im := range reImportFrom.FindAllStringSubmatch(src, -1) {
		if strings.Contains(im[1], factory) {
			clientsMod = im[2]
			break
		}
	}
	if clientsMod == "" {
		return "", factory + " 를 어디서 들여오는지 못 찾았다"
	}
	clientsPath := tsModulePath(repoPath, file, clientsMod)
	if clientsPath == "" {
		return "", clientsMod + " 를 저장소 안에서 못 찾았다"
	}

	cb, err := os.ReadFile(clientsPath)
	if err != nil {
		return "", ""
	}
	csrc := string(cb)

	// 그 공장의 정의 줄에서 계약 이름을 뽑는다
	defRe := regexp.MustCompile(`(?m)^\s*export\s+const\s+` + regexp.QuoteMeta(factory) + `\s*=.*?<\s*typeof\s+([\p{L}\p{N}_]+)`)
	dm := defRe.FindStringSubmatch(csrc)
	if dm == nil {
		return "", factory + " 의 정의에서 계약 이름을 못 읽었다"
	}
	defName := dm[1]

	// 그 이름을 들여오는 경로가 임자를 말해 준다
	rel, _ := filepath.Rel(repoPath, clientsPath)
	for _, im := range reImportFrom.FindAllStringSubmatch(csrc, -1) {
		if !importsName(im[1], defName) {
			continue
		}
		p := tsModulePath(repoPath, filepath.ToSlash(rel), im[2])
		if p == "" {
			continue
		}
		pr, _ := filepath.Rel(repoPath, p)
		if owner := protoOwnerRepo(filepath.ToSlash(pr)); owner != "" {
			return owner, fmt.Sprintf("%s() → %s → %s", factory, defName, filepath.ToSlash(pr))
		}
	}
	return "", defName + " 를 들여오는 곳이 생성물이 아니다"
}

// importsName 은 들여오기 목록에 그 이름이 있는지 본다. `X as Y` 의 Y 도 본다.
func importsName(list, want string) bool {
	for _, part := range strings.Split(list, ",") {
		p := strings.TrimSpace(part)
		if i := strings.Index(p, " as "); i >= 0 {
			p = strings.TrimSpace(p[i+4:])
		}
		if p == want {
			return true
		}
	}
	return false
}

// protoOwnerViaClientDeep 는 그 파일에서 못 따라가면 **그 파일이 들여오는
// 저장소 안 모듈**까지 한 걸음 더 간다.
//
// 계획이 짚는 파일은 회차마다 다르다. 화면 파일만 짚은 회차에서는 gRPC 를
// 부르는 곳이 없어 고리가 끊겼다 — 네 번 가운데 한 번이 그래서 임자를 못
// 찾았다(W-58547). 화면은 데이터 모듈을 들여오고, 그 모듈이 공장을 부른다.
// 한 걸음이면 닿는다.
func protoOwnerViaClientDeep(repoPath, file string, hops int) (string, string) {
	seen := map[string]bool{}
	cur := []string{file}
	for h := 0; h <= hops; h++ {
		var next []string
		for _, f := range cur {
			if seen[f] {
				continue
			}
			seen[f] = true
			if o, why := protoOwnerViaClient(repoPath, f); o != "" {
				return o, why
			}
			next = append(next, inRepoImports(repoPath, f)...)
		}
		if len(next) == 0 {
			break
		}
		cur = next
	}
	return "", "이 파일들과 그것들이 들여오는 모듈에서 gRPC 공장을 못 찾았다"
}

// inRepoImports 는 그 파일이 들여오는 **저장소 안** 모듈의 경로를 준다.
func inRepoImports(repoPath, file string) []string {
	b, err := os.ReadFile(filepath.Join(repoPath, file))
	if err != nil {
		return nil
	}
	var out []string
	for _, im := range reImportFrom.FindAllStringSubmatch(string(b), -1) {
		p := tsModulePath(repoPath, file, im[2])
		if p == "" {
			continue
		}
		if rel, err := filepath.Rel(repoPath, p); err == nil {
			out = append(out, filepath.ToSlash(rel))
		}
	}
	return out
}
