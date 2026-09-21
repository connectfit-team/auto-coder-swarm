package orchestrator

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// 저장소가 쓰는 계약은 **전부 적혀 있다.** 공장을 만드는 줄이 계약 이름을
// 들고 있고, 그 이름을 들여오는 경로가 생성물 자리를 말해 준다.
//
// 여태는 계획이 짚은 파일에서만 따라갔다. 그래서 계획이 화면 파일만 짚은
// 회차에서는 계약이 하나도 안 나왔고(W-71416), 엉뚱한 파일을 짚은 회차에서는
// 엉뚱한 계약이 나왔다. 계획이 무엇을 짚었든 목록은 같아야 한다.
var reClientDef = regexp.MustCompile(
	`(?m)^\s*export\s+const\s+(get[\p{L}\p{N}_]*Client)\s*=\s*[\p{L}\p{N}_.]+\s*<\s*typeof\s+([\p{L}\p{N}_]+)`)

// 훑지 않는 곳. 남의 코드와 빌드 결과물이다.
var skipDirs = map[string]bool{
	"node_modules": true, ".git": true, "dist": true, "build": true,
	".svelte-kit": true, "vendor": true, "coverage": true,
}

const (
	maxContractScanFiles = 5000
	maxContractFileBytes = 512 * 1024
)

// repoContracts 는 이 저장소가 쓰는 계약을 전부 준다. 열쇠는 생성물 경로다.
func repoContracts(repoPath string) map[string]tracedContract {
	out := map[string]tracedContract{}
	if repoPath == "" {
		return out
	}
	scanned := 0
	filepath.WalkDir(repoPath, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if scanned >= maxContractScanFiles {
			return filepath.SkipAll
		}
		if ext := filepath.Ext(p); ext != ".ts" && ext != ".tsx" {
			return nil
		}
		st, err := d.Info()
		if err != nil || st.Size() > maxContractFileBytes {
			return nil
		}
		scanned++
		b, err := os.ReadFile(p)
		if err != nil || !strings.Contains(string(b), "typeof ") {
			return nil
		}
		rel, err := filepath.Rel(repoPath, p)
		if err != nil {
			return nil
		}
		collectContracts(repoPath, filepath.ToSlash(rel), string(b), out)
		return nil
	})
	return out
}

// collectContracts 는 한 파일 안의 공장 정의에서 계약을 뽑는다.
func collectContracts(repoPath, rel, src string, out map[string]tracedContract) {
	defs := reClientDef.FindAllStringSubmatch(src, -1)
	if len(defs) == 0 {
		return
	}
	imports := reImportFrom.FindAllStringSubmatch(src, -1)
	for _, d := range defs {
		factory, defName := d[1], d[2]
		for _, im := range imports {
			if !importsName(im[1], defName) {
				continue
			}
			p := tsModulePath(repoPath, rel, im[2])
			if p == "" {
				continue
			}
			gen, err := filepath.Rel(repoPath, p)
			if err != nil {
				continue
			}
			gen = filepath.ToSlash(gen)
			owner := protoOwnerRepo(gen)
			if owner == "" {
				continue
			}
			if _, dup := out[gen]; dup {
				continue
			}
			out[gen] = tracedContract{owner: owner, why: factory + "() → " + defName + " → " + gen}
			break
		}
	}
}
