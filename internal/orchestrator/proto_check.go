package orchestrator

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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
	out = append(out, newServiceBesideOne(repoPath, files, diff)...)
	out = append(out, fieldNumberGaps(repoPath, files, diff)...)
	out = append(out, enumZeroMeansSomething(diff)...)
	out = append(out, sameNameDifferentType(diff)...)
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

var (
	reServiceDecl = regexp.MustCompile(`(?m)^\s*service\s+(\w+)`)
	reEnumOpen    = regexp.MustCompile(`(?m)^\s*enum\s+(\w+)\s*\{`)
	reEnumValue   = regexp.MustCompile(`(?m)^\s*(\w+)\s*=\s*(\d+)\s*[;\[]`)
)

// newServiceBesideOne 은 이미 service 가 있는 파일에 service 를 새로 만들었는지 본다.
//
// 자식이 되풀이해 이렇게 했다 — 있는 service Internal 에 rpc 를 더하는 대신
// service ConnectService 를 새로 만들고 그 안에 넣었다. 쓰는 쪽은 Internal 을
// 부르므로 그 rpc 는 아무도 못 부른다. 한 번은 새 service 안에
// 「// 기존 RPC …」 라는 자리표시까지 남겼다.
func newServiceBesideOne(repoPath string, changed []string, diff string) []guard.Violation {
	added := map[string]bool{}
	for _, line := range strings.Split(diff, "\n") {
		if !strings.HasPrefix(line, "+") || strings.HasPrefix(line, "+++") {
			continue
		}
		if m := reServiceDecl.FindStringSubmatch(line[1:]); m != nil {
			added[m[1]] = true
		}
	}
	if len(added) == 0 {
		return nil
	}
	var out []guard.Violation
	for _, rel := range changed {
		b, err := os.ReadFile(filepath.Join(repoPath, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		var existing []string
		for _, m := range reServiceDecl.FindAllStringSubmatch(string(b), -1) {
			if !added[m[1]] {
				existing = append(existing, m[1])
			}
		}
		if len(existing) == 0 {
			continue
		}
		for name := range added {
			out = append(out, guard.Violation{
				Why: fmt.Sprintf("service %s 를 새로 만들었다 — 이 파일에는 이미 service %s 가 있다",
					name, strings.Join(existing, " · ")),
				Evidence: []string{rel, "있는 service 안에 rpc 를 더해라. 새 service 는 쓰는 쪽이 부르지 않는다"},
			})
		}
	}
	return out
}

// fieldNumberGaps 는 새 필드 번호가 멀리 뛰었는지 본다.
//
// 「번호는 기존 필드와 충돌되지 않도록 선택」 이라며 14 다음에 99 를 쓴 수정이
// 있었다. 겹치지는 않지만 그 사이 번호가 통째로 막히고, 다음 사람이 무엇을
// 쓸 수 있는지 알 수 없다. 빈 다음 번호를 쓴다.
func fieldNumberGaps(repoPath string, changed []string, diff string) []guard.Violation {
	addedNums := map[string]bool{}
	for _, line := range strings.Split(diff, "\n") {
		if !strings.HasPrefix(line, "+") || strings.HasPrefix(line, "+++") {
			continue
		}
		if m := reProtoField.FindStringSubmatch(line[1:]); m != nil {
			addedNums[m[1]+"="+m[2]] = true
		}
	}
	if len(addedNums) == 0 {
		return nil
	}
	var out []guard.Violation
	for _, rel := range changed {
		b, err := os.ReadFile(filepath.Join(repoPath, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		for _, blk := range messageBlocks(string(b)) {
			var old, added []int
			for _, f := range reProtoField.FindAllStringSubmatch(blk.body, -1) {
				n, err := strconv.Atoi(f[2])
				if err != nil {
					continue
				}
				if addedNums[f[1]+"="+f[2]] {
					added = append(added, n)
				} else {
					old = append(old, n)
				}
			}
			if len(added) == 0 || len(old) == 0 {
				continue
			}
			max := 0
			for _, n := range old {
				if n > max {
					max = n
				}
			}
			for _, n := range added {
				if n > max+len(added) {
					out = append(out, guard.Violation{
						Why: fmt.Sprintf("%s 에 번호 %d 을 썼다 — 있던 마지막이 %d 이니 %d 부터 쓴다",
							blk.name, n, max, max+1),
						Evidence: []string{rel, "사이 번호가 통째로 막히고, 다음 사람이 무엇을 쓸 수 있는지 알 수 없다"},
					})
				}
			}
		}
	}
	return out
}

// enumZeroMeansSomething 은 새 enum 의 0 번이 뜻을 가진 값인지 본다.
//
// `enum PendingState { PENDING = 0; … }` 를 더한 수정이 있었다. proto3 에서
// 0 은 값이 없을 때의 기본값이라, 그러면 **여태 있던 모든 요청이 보류로
// 읽힌다.** 0 번은 「정해지지 않음」 이어야 한다.
func enumZeroMeansSomething(diff string) []guard.Violation {
	var out []guard.Violation
	var cur string
	depth := 0
	for _, line := range strings.Split(diff, "\n") {
		if !strings.HasPrefix(line, "+") || strings.HasPrefix(line, "+++") {
			continue
		}
		body := line[1:]
		if m := reEnumOpen.FindStringSubmatch(body); m != nil {
			cur, depth = m[1], 1
			continue
		}
		if cur == "" {
			continue
		}
		if strings.Contains(body, "}") {
			depth--
			if depth <= 0 {
				cur = ""
			}
			continue
		}
		m := reEnumValue.FindStringSubmatch(body)
		if m == nil || m[2] != "0" {
			continue
		}
		name := strings.ToUpper(m[1])
		ok := false
		for _, w := range []string{"UNSPECIFIED", "UNKNOWN", "NONE", "INVALID", "DEFAULT"} {
			if strings.Contains(name, w) {
				ok = true
			}
		}
		if !ok {
			out = append(out, guard.Violation{
				Why:      fmt.Sprintf("enum %s 의 0 번이 %s 다 — 0 은 값이 없을 때의 기본값이라 여태 있던 것이 모두 그 값으로 읽힌다", cur, m[1]),
				Evidence: []string{"0 번은 UNSPECIFIED 처럼 「정해지지 않음」 이어야 한다"},
			})
		}
		cur = ""
	}
	return out
}

// reFieldTyped 는 더한 필드의 타입과 이름을 함께 읽는다.
var reFieldTyped = regexp.MustCompile(`^\s*(?:repeated\s+|optional\s+)?([\w.]+)\s+(\w+)\s*=\s*\d+\s*[;\[]`)

// sameNameDifferentType 은 한 수정 안에서 같은 것을 두 타입으로 나타냈는지 본다.
//
// 실측: ReceivedRequest 에 `RequestPendingStatus pending_status` 를 더해
// 놓고, 그 상태를 바꾸는 RPC 의 요청에는 `string new_status` 를 받았다.
// 같은 것을 한쪽은 enum 으로, 한쪽은 글자로 다룬다 — 쓰는 쪽이 둘을 손으로
// 맞춰야 하고, 맞추지 않으면 조용히 어긋난다.
//
// 이름의 마지막 토막(…_status)이 같은데 타입이 다르면 짚는다.
func sameNameDifferentType(diff string) []guard.Violation {
	types := map[string]map[string]bool{} // 이름 꼬리 → 타입들
	for _, line := range strings.Split(diff, "\n") {
		if !strings.HasPrefix(line, "+") || strings.HasPrefix(line, "+++") {
			continue
		}
		m := reFieldTyped.FindStringSubmatch(line[1:])
		if m == nil {
			continue
		}
		typ, name := m[1], m[2]
		parts := strings.Split(name, "_")
		tail := parts[len(parts)-1]
		if len(tail) < 3 {
			continue // id·no 처럼 짧은 꼬리는 겹쳐도 뜻이 없다
		}
		if types[tail] == nil {
			types[tail] = map[string]bool{}
		}
		types[tail][typ] = true
	}
	var out []guard.Violation
	for tail, ts := range types {
		if len(ts) < 2 {
			continue
		}
		// 하나라도 만든 타입(글자가 아닌 것)이 섞여 있을 때만 본다.
		named := false
		var list []string
		for t := range ts {
			list = append(list, t)
			if !protoScalars[t] {
				named = true
			}
		}
		if !named {
			continue
		}
		sort.Strings(list)
		out = append(out, guard.Violation{
			Why: fmt.Sprintf("…_%s 를 서로 다른 타입으로 다룬다 (%s)", tail, strings.Join(list, " · ")),
			Evidence: []string{
				"같은 것을 한쪽은 만든 타입으로, 한쪽은 글자로 다루면 쓰는 쪽이 손으로 맞춰야 한다"},
		})
	}
	return out
}

// proto 의 기본 타입.
var protoScalars = map[string]bool{
	"double": true, "float": true, "int32": true, "int64": true,
	"uint32": true, "uint64": true, "sint32": true, "sint64": true,
	"fixed32": true, "fixed64": true, "sfixed32": true, "sfixed64": true,
	"bool": true, "string": true, "bytes": true,
}
