package orchestrator

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// 없는 이름이 **누구 것인지** 가린다.
//
// #74 는 "없는 이름" 을 전부 계약 구멍으로 봤다. 그런데 W-14287 이 실제로
// 낸 것은 이랬다.
//
//	Cannot find name 'pending'                                  ← 이 파일의 실수
//	Property 'workplaceId' does not exist on type 'StaffSummary' ← ?
//	Property 'hold' does not exist on type 'ConnectableStaff'    ← 계약
//
// 셋을 한 덩이로 보면 남의 저장소에 엉뚱한 일을 만든다. 갈라야 한다.
//
// 가르는 법은 **타입이 정의된 자리**다. ConnectableStaff·StaffSummary·
// LaborContract 는 전부 `src/lib/server/protos/ceowebapis/...` 에 있다 —
// 저장소 안에 있지만 **생성물**이다. 생성물은 손으로 고치지 않는다. 그
// 필드가 없다는 것은 proto 에 없다는 뜻이고, 고칠 곳은 proto 저장소다.
//
// 게다가 경로가 임자를 말해 준다. `.../protos/ceowebapis/...` → proto-ceowebapis.
// 모델에게 묻지 않는다. 경로를 읽는다.

var (
	reTypeDecl  = regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:declare\s+)?(?:type|interface|class|enum)\s+(\w+)\b`)
	reProtosDir = regexp.MustCompile(`(?:^|/)protos?/([A-Za-z0-9_.-]+)/`)
)

// typeHome 은 그 타입이 정의된 파일이다. 못 찾으면 빈 문자열이다.
func typeHome(repoPath, typeName string) string {
	if typeName == "" {
		return ""
	}
	want := regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:declare\s+)?(?:type|interface|class|enum)\s+` +
		regexp.QuoteMeta(typeName) + `\b`)
	found := ""
	_ = filepath.WalkDir(filepath.Join(repoPath, "src"), func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || found != "" {
			return nil
		}
		switch filepath.Ext(p) {
		case ".ts", ".d.ts", ".svelte":
		default:
			return nil
		}
		b, rerr := os.ReadFile(p)
		if rerr == nil && want.Match(b) {
			rel, _ := filepath.Rel(repoPath, p)
			found = filepath.ToSlash(rel)
		}
		return nil
	})
	return found
}

// protoOwnerRepo 는 생성물 경로에서 그 계약을 펴내는 저장소 이름을 읽는다.
// 생성물이 아니면 빈 문자열이다.
func protoOwnerRepo(path string) string {
	if path == "" || !isGeneratedPath(path) {
		return ""
	}
	m := reProtosDir.FindStringSubmatch(filepath.ToSlash(path))
	if m == nil {
		return ""
	}
	name := strings.TrimSuffix(m[1], ".git")
	if name == "" {
		return ""
	}
	if strings.HasPrefix(name, "proto-") {
		return name
	}
	// 생성물 칸 이름이 곧 저장소 이름이다. 그 관행(…apis)을 벗어난 칸까지
	// 받으면 protos/google/… 이 proto-google 이라는 없는 저장소가 된다.
	if !strings.HasSuffix(name, "apis") {
		return ""
	}
	return "proto-" + name
}

// splitMissing 은 없는 이름을 **계약 구멍**과 **이 저장소의 실수**로 가른다.
// 계약 구멍은 (저장소이름 → 이름들) 로 묶어 준다.
func splitMissing(repoPath string, names []string) (contract map[string][]string, local []string) {
	contract = map[string][]string{}
	for _, n := range names {
		typeName, field := n, ""
		if i := strings.LastIndex(n, "."); i > 0 {
			typeName, field = n[:i], n[i+1:]
		}
		home := typeHome(repoPath, typeName)
		owner := protoOwnerRepo(home)
		if owner == "" || field == "" {
			// 타입을 못 찾았거나 생성물이 아니면 이 저장소의 일이다.
			local = append(local, n)
			continue
		}
		contract[owner] = append(contract[owner], n)
	}
	return contract, local
}
