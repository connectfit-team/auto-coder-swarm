package orchestrator

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// 타입의 **필드 이름**까지 보여 준다.
//
// 모듈 목록만으로는 이 잘못을 막을 수 없었다(W-51535).
//
//	Property 'workplaceId' does not exist on type 'StaffSummary'.
//	  Did you mean 'workPlaceId'?
//	Property 'workPlaceId' does not exist on type 'ConnectableStaff'.
//	  Did you mean 'workplaceId'?
//
// 두 타입이 **같은 개념을 다른 표기로** 쓴다. StaffSummary 는 workPlaceId,
// ConnectableStaff 는 workplaceId 다. 모델은 둘 사이를 오가며 하나를 고치고
// 다른 하나를 깨뜨렸다 — 회차마다 오류 수가 같아 두 번 만에 멈췄다.
//
// 지어낼 수 없게 하려면 있는 것을 보여 주면 된다. 파일이 실제로 쓰는 타입만
// 골라 그 필드를 적는다.

var (
	reTypeBlock = regexp.MustCompile(`(?s)(?:export\s+)?(?:declare\s+)?(?:interface|type)\s+%s\s*(?:=\s*)?\{(.*?)\n\}`)
	reFieldName = regexp.MustCompile(`(?m)^\s*(?:readonly\s+)?([\p{L}_][\p{L}\p{N}_]*)\s*\??\s*:`)
)

// typeFields 는 그 타입의 필드 이름을 준다. 못 찾으면 빈 목록이다.
func typeFields(repoPath, typeName string) []string {
	home := typeHome(repoPath, typeName)
	if home == "" {
		return nil
	}
	b, err := os.ReadFile(filepath.Join(repoPath, home))
	if err != nil {
		return nil
	}
	re := regexp.MustCompile(strings.Replace(reTypeBlock.String(), "%s", regexp.QuoteMeta(typeName), 1))
	m := re.FindSubmatch(b)
	if m == nil {
		return nil
	}
	seen := map[string]bool{}
	for _, f := range reFieldName.FindAllSubmatch(m[1], -1) {
		seen[string(f[1])] = true
	}
	return sortedKeys(seen)
}

// typeFieldSheetFor 는 **들여온 모듈이 내보내는 타입**의 필드를 적는다.
//
// 처음에는 "이 파일에 나오는 대문자 이름" 으로 골랐는데 빗나갔다. 파일은
// 타입 이름을 직접 쓰지 않는다 — `listConnectableStaff(...)` 처럼 함수로만
// 부르고 타입은 추론으로 붙는다. 낱말 경계 규칙은 `listConnectableStaff`
// 안의 `ConnectableStaff` 를 (옳게) 거르므로 후보에 아예 못 들었다.
//
// 이름의 출처는 이미 있다 — 쪽지가 모듈마다 내보내는 이름을 세어 두었다.
// 그중 타입인 것만 골라 필드를 적는다.
func typeFieldSheetFor(repoPath string, candidates []string, max int) string {
	seen := map[string]bool{}
	var lines []string
	for _, n := range candidates {
		if len(lines) >= max {
			break
		}
		if n == "" || seen[n] || n[0] < 'A' || n[0] > 'Z' {
			continue // 타입은 대문자로 시작한다
		}
		seen[n] = true
		if fs := typeFields(repoPath, n); len(fs) > 0 {
			lines = append(lines, "    "+n+" → "+strings.Join(clipNames(fs, 30), ", "))
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return "  이 파일이 쓰는 타입의 필드(표기까지 그대로 써라 — 비슷한 이름을 지어내지 마라)\n" +
		strings.Join(lines, "\n") + "\n"
}
