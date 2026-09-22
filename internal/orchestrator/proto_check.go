package orchestrator

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/guard"
)

// 빌드는 계약을 못 본다.
//
// 계약(.proto)만 고치면 생성물(*.pb.go)은 그대로이므로 `go build` 가 통과한다.
// 그래서 이미 있는 메시지를 **다른 파일에 통째로 다시 정의한** 수정이 승인
// 대기까지 갔다(W-36969: ReceivedRequest 를 필드 4개짜리로 새로 만들었다.
// 진짜는 다른 파일에 필드 14개로 있다). protoc 이라면 중복 정의로 깨진다.
//
// protoc 는 돌리지 않는다(펴내기는 make push-*apis 만 쓴다). 대신 계약을
// 읽어 **기계가 셀 수 있는 것**만 본다 — 중복 정의, 겹치는 필드 번호,
// 있던 것을 고쳤는지.
var (
	reProtoPackage = regexp.MustCompile(`(?m)^\s*package\s+([\w.]+)\s*;`)
	reProtoDecl    = regexp.MustCompile(`(?m)^\s*(message|enum|service)\s+([A-Za-z_]\w*)`)
	reProtoField   = regexp.MustCompile(`(?m)^\s*(?:repeated\s+|optional\s+|required\s+)?[\w.]+\s+(\w+)\s*=\s*(\d+)\s*[;\[]`)
	reProtoRPC     = regexp.MustCompile(`(?m)^\s*rpc\s+(\w+)\s*\(`)
)

// CheckProtoChange 는 계약 수정이 조용히 깨뜨리는 것을 잡는다.
func CheckProtoChange(repoPath, diff string) []guard.Violation {
	files := protoFilesInDiff(diff)
	if len(files) == 0 {
		return nil
	}
	var out []guard.Violation
	out = append(out, duplicateDecls(repoPath, files)...)
	out = append(out, duplicateFieldNumbers(repoPath, files)...)
	out = append(out, changedExisting(diff)...)
	return out
}

// protoFilesInDiff 는 diff 가 고친 .proto 경로를 준다.
func protoFilesInDiff(diff string) []string {
	var out []string
	for _, line := range strings.Split(diff, "\n") {
		if !strings.HasPrefix(line, "+++") {
			continue
		}
		f := strings.TrimPrefix(strings.Fields(line + " ")[1], "b/")
		if strings.HasSuffix(f, ".proto") {
			out = append(out, f)
		}
	}
	return out
}

// duplicateDecls 는 같은 proto 꾸러미 안에서 이름이 두 번 선언됐는지 본다.
func duplicateDecls(repoPath string, changed []string) []guard.Violation {
	type where struct{ file string }
	seen := map[string]map[string][]where{} // 꾸러미 → 이름 → 자리들

	forEachProto(repoPath, func(rel, src string) {
		pkg := "(이름 없음)"
		if m := reProtoPackage.FindStringSubmatch(src); m != nil {
			pkg = m[1]
		}
		if seen[pkg] == nil {
			seen[pkg] = map[string][]where{}
		}
		for _, d := range reProtoDecl.FindAllStringSubmatch(src, -1) {
			seen[pkg][d[2]] = append(seen[pkg][d[2]], where{rel})
		}
	})

	touched := map[string]bool{}
	for _, f := range changed {
		touched[f] = true
	}
	var out []guard.Violation
	for pkg, names := range seen {
		for name, ws := range names {
			if len(ws) < 2 {
				continue
			}
			hit := false
			var files []string
			for _, w := range ws {
				files = append(files, w.file)
				if touched[w.file] {
					hit = true
				}
			}
			if !hit {
				continue // 이번 수정과 무관한 중복은 이 일의 몫이 아니다
			}
			out = append(out, guard.Violation{
				Why: fmt.Sprintf("%s 를 두 번 선언한다 — 계약 꾸러미 %s 안에서 이름은 하나여야 한다", name, pkg),
				Evidence: []string{strings.Join(files, " · "),
					"이미 있는 것을 고치고, 새로 만들지 마라"},
			})
		}
	}
	return out
}

// duplicateFieldNumbers 는 한 메시지 안에서 번호가 겹치는지 본다.
func duplicateFieldNumbers(repoPath string, changed []string) []guard.Violation {
	var out []guard.Violation
	for _, rel := range changed {
		b, err := os.ReadFile(filepath.Join(repoPath, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		for _, blk := range messageBlocks(string(b)) {
			nums := map[int]string{}
			for _, f := range reProtoField.FindAllStringSubmatch(blk.body, -1) {
				n, err := strconv.Atoi(f[2])
				if err != nil {
					continue
				}
				if prev, dup := nums[n]; dup {
					out = append(out, guard.Violation{
						Why: fmt.Sprintf("%s 의 필드 번호 %d 가 겹친다 (%s · %s)", blk.name, n, prev, f[1]),
						Evidence: []string{rel,
							"번호는 한 메시지 안에서 하나뿐이어야 한다 — 겹치면 옛 데이터가 다른 필드로 읽힌다"},
					})
					continue
				}
				nums[n] = f[1]
			}
		}
	}
	return out
}

// changedExisting 은 있던 필드·RPC 를 고치거나 지웠는지 본다.
func changedExisting(diff string) []guard.Violation {
	var removed []string
	for _, line := range strings.Split(diff, "\n") {
		if !strings.HasPrefix(line, "-") || strings.HasPrefix(line, "---") {
			continue
		}
		body := line[1:]
		if m := reProtoField.FindStringSubmatch(body); m != nil {
			removed = append(removed, fmt.Sprintf("필드 %s = %s", m[1], m[2]))
			continue
		}
		if m := reProtoRPC.FindStringSubmatch(body); m != nil {
			removed = append(removed, "rpc "+m[1])
		}
	}
	if len(removed) == 0 {
		return nil
	}
	return []guard.Violation{{
		Why:      "있던 필드·RPC 를 고치거나 지웠다 — 계약은 더하기만 한다",
		Evidence: append(clipList(removed), "쓰는 쪽이 조용히 깨진다. 새 번호로 더해라"),
	}}
}

type protoBlock struct{ name, body string }

// messageBlocks 는 message 블록을 이름과 몸통으로 가른다.
func messageBlocks(src string) []protoBlock {
	var out []protoBlock
	for _, loc := range reProtoDecl.FindAllStringSubmatchIndex(src, -1) {
		kind := src[loc[2]:loc[3]]
		if kind != "message" {
			continue
		}
		name := src[loc[4]:loc[5]]
		depth, start := 0, -1
		for i := loc[1]; i < len(src); i++ {
			switch src[i] {
			case '{':
				if depth == 0 {
					start = i + 1
				}
				depth++
			case '}':
				depth--
				if depth == 0 {
					out = append(out, protoBlock{name, src[start:i]})
					i = len(src)
				}
			}
		}
	}
	return out
}

// forEachProto 는 저장소 안 .proto 를 하나씩 준다.
func forEachProto(repoPath string, fn func(rel, src string)) {
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
		if !strings.HasSuffix(d.Name(), ".proto") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(repoPath, p)
		if err != nil {
			return nil
		}
		fn(filepath.ToSlash(rel), string(b))
		return nil
	})
}
