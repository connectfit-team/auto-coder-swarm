package orchestrator

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// 저장소가 쓰는 계약은 전부 적혀 있다. 공장을 만드는 줄이 계약 이름을 들고
// 있고, 그 이름을 들여오는 경로가 생성물 자리를 말해 준다.
//
// 계획이 짚은 파일에서만 따라가면 계획이 빗나간 회차에 후보가 통째로
// 사라지거나 엉뚱한 것만 남는다. 목록은 계획과 무관해야 한다.
var (
	// export const x = 무엇<typeof XDefinition, …>
	reClientGeneric = regexp.MustCompile(`(?m)^[ \t]*export[ \t]+const[ \t]+([\p{L}\p{N}_]+)[ \t]*=[ \t]*[\p{L}\p{N}_.]+[ \t]*<\s*typeof[ \t]+([\p{L}\p{N}_]+)`)
	// export const x = 무엇(XDefinition, …)
	reClientArg = regexp.MustCompile(`(?m)^[ \t]*export[ \t]+const[ \t]+([\p{L}\p{N}_]+)[ \t]*=[ \t]*[\p{L}\p{N}_.]+[ \t]*\(\s*([\p{L}\p{N}_]*Definition)\b`)
)

// 훑지 않는 곳. 남의 코드와 빌드 결과물이다.
var skipDirs = map[string]bool{
	"node_modules": true, ".git": true, "dist": true, "build": true,
	".svelte-kit": true, "vendor": true, "coverage": true,
}

const (
	maxContractScanFiles = 5000
	maxContractFileBytes = 512 * 1024
	maxContractNote      = 160
	maxContractNoteLines = 40 // 긴 주석은 앞부터 싣는다 — 끝만 잡으면 문장 중간부터 나온다
)

// contractScan 은 훑은 결과다. **얼마나 훑었는지도 함께 준다** — 상한에
// 걸려 잘린 목록을 온전한 것처럼 내놓으면 「목록에 없다」 가 「이 저장소에
// 없다」 로 읽힌다.
type contractScan struct {
	contracts map[string]tracedContract
	scanned   int      // 실제로 읽은 파일 수
	factories int      // 공장 정의를 본 횟수 (관행은 읽혔다는 뜻)
	truncated bool     // 상한에 걸려 끝까지 못 갔다
	skipped   []string // 너무 커서 건너뛴 파일
}

// note 는 사람이 볼 한 줄이다. 온전히 훑었으면 빈 문자열이다.
func (s contractScan) note() string {
	switch {
	case s.truncated:
		return "파일 상한에 걸려 끝까지 훑지 못했다 — 목록이 온전하지 않다"
	case len(s.contracts) == 0 && s.factories > 0:
		return "공장 정의는 봤지만 계약 경로를 못 따라갔다 — 이 저장소의 들여오기 관행을 못 읽었다"
	case len(s.contracts) == 0:
		return "이 저장소에서 gRPC 공장 정의를 찾지 못했다"
	}
	if len(s.skipped) > 0 {
		return "너무 커서 건너뛴 파일이 있다: " + strings.Join(s.skipped, ", ")
	}
	return ""
}

// repoContracts 는 이 저장소가 쓰는 계약을 준다. 열쇠는 생성물 경로다.
func repoContracts(repoPath string) contractScan {
	out := contractScan{contracts: map[string]tracedContract{}}
	if repoPath == "" {
		return out
	}
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
		if ext := filepath.Ext(p); ext != ".ts" && ext != ".tsx" {
			return nil
		}
		if out.scanned >= maxContractScanFiles {
			out.truncated = true
			return filepath.SkipAll
		}
		st, err := d.Info()
		if err != nil {
			return nil
		}
		rel, relErr := filepath.Rel(repoPath, p)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if st.Size() > maxContractFileBytes {
			out.skipped = append(out.skipped, rel)
			return nil
		}
		out.scanned++
		b, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		collectContracts(repoPath, rel, string(b), &out)
		return nil
	})
	return out
}

// collectContracts 는 한 파일 안의 공장 정의에서 계약을 뽑는다.
func collectContracts(repoPath, rel, src string, out *contractScan) {
	var hits [][]int
	hits = append(hits, reClientGeneric.FindAllStringSubmatchIndex(src, -1)...)
	hits = append(hits, reClientArg.FindAllStringSubmatchIndex(src, -1)...)
	if len(hits) == 0 {
		return
	}
	out.factories += len(hits)
	imports := reImportFrom.FindAllStringSubmatch(src, -1)
	for _, h := range hits {
		factory := src[h[2]:h[3]]
		defName := src[h[4]:h[5]]
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
			if _, dup := out.contracts[gen]; dup {
				continue
			}
			out.contracts[gen] = tracedContract{
				owner:    owner,
				contract: gen,
				why:      factory + "() → " + defName + " → " + gen,
				note:     docCommentAbove(src, h[0]),
			}
			break
		}
	}
}

// docCommentAbove 는 그 선언 바로 위에 달린 말을 준다.
//
// 「앱과 같은 RPC 다」·「조회에는 쓰지 마라」 처럼 **고르면 안 되는 계약**임을
// 적어 둔 곳이 거기다. 목록에서 그것이 빠지면 쓰기 계약과 구별되지 않는다.
func docCommentAbove(src string, at int) string {
	lines := strings.Split(src[:at], "\n")
	var got []string
	for i := len(lines) - 1; i >= 0 && len(got) < maxContractNoteLines; i-- {
		s := strings.TrimSpace(lines[i])
		if s == "" && len(got) == 0 {
			continue
		}
		switch {
		case strings.HasPrefix(s, "//"):
			s = strings.TrimSpace(strings.TrimPrefix(s, "//"))
		case strings.HasPrefix(s, "*/"), strings.HasPrefix(s, "/*"), strings.HasPrefix(s, "*"):
			s = strings.TrimSpace(strings.Trim(s, "/*"))
		default:
			i = -1
			continue
		}
		if s != "" {
			got = append([]string{s}, got...)
		}
	}
	return clip(strings.Join(got, " "), maxContractNote)
}
