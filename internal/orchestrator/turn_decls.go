package orchestrator

import (
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"
)

var reTurnDecl = regexp.MustCompile(`^\s*(message|enum|service)\s+([A-Za-z_]\w*)`)

// 코더는 파일을 하나씩 고치면서 **직전에 제가 무엇을 썼는지 모른다.**
//
// 실측으로 같은 이름을 두 파일에 두 번 만들었다.
//
//	W-96045  message RequestUpdateRequest 를 communication·service 양쪽에
//	W-11906  enum HoldStatus 를 communication·service 양쪽에
//
// protoc 이라면 중복 정의로 깨진다. `go build` 는 생성물을 다시 만들지 않으니
// 통과하고, 비평가나 관문이 뒤늦게 잡는다. 그때는 이미 세 시도 가운데 하나를
// 버린 뒤다 — W-11906 은 그 되돌이에서 RPC 를 통째로 잃었다.
//
// 이름을 다 보여 주는 것으로는 못 고친다. 이 꾸러미의 선언이 263개, 6,106자다.
// **이번 턴에 제가 더한 것만** 보여 주면 된다. hermes·aider 가 턴 누적 diff 를
// 들고 다니는 것과 같은 까닭이다.

// declsAddedThisTurn 은 작업 사본에서 **이번에 새로 더한** proto 선언을 모은다.
//
// git diff 로 읽는다 — 코더가 무엇을 썼는지는 파일이 안다. 따로 장부를 들면
// 그 장부가 어긋난다.
func declsAddedThisTurn(repoPath string) map[string]string {
	out := map[string]string{} // 이름 → 어느 파일에
	if repoPath == "" {
		return out
	}
	b, err := exec.Command("git", "-C", repoPath, "diff", "--unified=0", "--", "*.proto").Output()
	if err != nil {
		return out
	}
	file := ""
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "+++ b/") {
			file = strings.TrimPrefix(line, "+++ b/")
			continue
		}
		if !strings.HasPrefix(line, "+") || strings.HasPrefix(line, "+++") {
			continue
		}
		if m := reTurnDecl.FindStringSubmatch(line[1:]); m != nil {
			if _, seen := out[m[2]]; !seen {
				out[m[2]] = file
			}
		}
	}
	return out
}

// alreadyAddedNote 는 이번에 더한 선언을 코더에게 일러 준다.
//
// 지금 고치는 파일에 든 것은 빼지 않는다 — 같은 파일에 두 번 쓰는 것도 막아야
// 한다.
func alreadyAddedNote(added map[string]string, editing string) string {
	if len(added) == 0 {
		return ""
	}
	names := make([]string, 0, len(added))
	for n := range added {
		names = append(names, n)
	}
	sort.Strings(names)

	var b strings.Builder
	b.WriteString("\n[이번에 이미 더한 선언]\n")
	for _, n := range names {
		where := added[n]
		if where == editing {
			where += " (지금 고치는 파일)"
		}
		fmt.Fprintf(&b, "  %s — %s\n", n, where)
	}
	b.WriteString("**이 이름들을 다시 선언하지 마라.** 한 proto 꾸러미에서 이름은 하나뿐이라, " +
		"두 번 쓰면 펴낼 때 중복 정의로 깨진다. 그 이름이 필요하면 그대로 **쓰기만** 하면 된다.\n\n")
	return b.String()
}
