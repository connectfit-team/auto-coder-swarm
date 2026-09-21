package orchestrator

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// 손으로 쓴 타입이어도 그 데이터가 어디서 오는지는 파일에 적혀 있다.
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

// protoContractsInFile 은 그 파일이 부르는 공장을 **모두** 따라가 계약을 준다.
//
// 첫 공장 하나만 보면, 계약 둘을 쓰는 파일이 하나로 접히고 어느 쪽이 뽑히는지가
// 파일 안 글자 순서로 정해진다.
func protoContractsInFile(repoPath, file string) ([]tracedContract, string) {
	b, err := os.ReadFile(filepath.Join(repoPath, file))
	if err != nil {
		return nil, ""
	}
	src := string(b)

	calls := reClientCall.FindAllStringSubmatch(src, -1)
	if len(calls) == 0 {
		return nil, "이 파일은 gRPC 공장을 부르지 않는다"
	}
	imports := reImportFrom.FindAllStringSubmatch(src, -1)

	var out []tracedContract
	var lastWhyNot string
	seen := map[string]bool{}
	for _, c := range calls {
		factory := c[1]
		if seen[factory] {
			continue
		}
		seen[factory] = true
		got, whyNot := contractOfFactory(repoPath, file, src, imports, factory)
		if got == nil {
			lastWhyNot = whyNot
			continue
		}
		out = append(out, *got)
	}
	if len(out) == 0 {
		return nil, lastWhyNot
	}
	return out, ""
}

// contractOfFactory 는 공장 하나를 계약까지 따라간다.
func contractOfFactory(repoPath, file, src string, imports [][]string, factory string) (*tracedContract, string) {
	var clientsMod string
	for _, im := range imports {
		if strings.Contains(im[1], factory) {
			clientsMod = im[2]
			break
		}
	}
	if clientsMod == "" {
		return nil, factory + " 를 어디서 들여오는지 못 찾았다"
	}
	clientsPath := tsModulePath(repoPath, file, clientsMod)
	if clientsPath == "" {
		return nil, clientsMod + " 를 저장소 안에서 못 찾았다"
	}
	cb, err := os.ReadFile(clientsPath)
	if err != nil {
		return nil, ""
	}
	csrc := string(cb)

	defRe := regexp.MustCompile(`(?m)^\s*export\s+const\s+` + regexp.QuoteMeta(factory) + `\s*=.*?<\s*typeof\s+([\p{L}\p{N}_]+)`)
	dm := defRe.FindStringSubmatch(csrc)
	if dm == nil {
		argRe := regexp.MustCompile(`(?m)^\s*export\s+const\s+` + regexp.QuoteMeta(factory) + `\s*=\s*[\p{L}\p{N}_.]+\s*\(\s*([\p{L}\p{N}_]*Definition)\b`)
		if dm = argRe.FindStringSubmatch(csrc); dm == nil {
			return nil, factory + " 의 정의에서 계약 이름을 못 읽었다"
		}
	}
	defName := dm[1]

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
		gen := filepath.ToSlash(pr)
		if o := protoOwnerRepo(gen); o != "" {
			return &tracedContract{
				owner:    o,
				contract: gen,
				why:      factory + "() → " + defName + " → " + gen,
			}, ""
		}
	}
	return nil, defName + " 를 들여오는 곳이 생성물이 아니다"
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

// protoContractsDeep 은 그 파일에서 못 따라가면 그 파일이 들여오는 저장소 안
// 모듈까지 한 걸음 더 간다.
//
// 계획이 짚는 파일은 회차마다 다르다. 화면 파일만 짚은 회차에서는 gRPC 를
// 부르는 곳이 없어 고리가 끊겼다. 화면은 데이터 모듈을 들여오고, 그 모듈이
// 공장을 부른다.
func protoContractsDeep(repoPath, file string, hops int) ([]tracedContract, string) {
	seen := map[string]bool{}
	cur := []string{file}
	for h := 0; h <= hops; h++ {
		var next []string
		for _, f := range cur {
			if seen[f] {
				continue
			}
			seen[f] = true
			if got, _ := protoContractsInFile(repoPath, f); len(got) > 0 {
				return got, ""
			}
			next = append(next, inRepoImports(repoPath, f)...)
		}
		if len(next) == 0 {
			break
		}
		cur = next
	}
	return nil, "이 파일들과 그것들이 들여오는 모듈에서 gRPC 공장을 못 찾았다"
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
