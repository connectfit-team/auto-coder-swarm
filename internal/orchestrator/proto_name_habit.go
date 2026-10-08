package orchestrator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/guard"
)

// suffixNameOffHabit 은 **이 계약이 안 쓰는 이름꼴**로 새 메시지를 만든 것을 잡는다.
//
// 실측 W-10104 은 관문을 다 지나 승인 대기까지 갔는데 판정 5에서 떨어졌다.
//
//	message UpdateReceivedRequestPendingStatusRequest { … }
//	message UpdateReceivedRequestPendingStatusResponse { … }
//
// 이 계약은 `RequestXxx`·`ResponseXxx` **접두형**이다. 실제로 세어 보니 접두형
// 203개, 접미형 **0개**다. 판정기는 이것을 보는데 관문은 안 봤다 — 조용히
// 통과해서 사람 손에 반쯤 틀린 것이 갔다.
//
// **규칙을 박아 두지 않는다.** 저장소가 실제로 쓰는 꼴을 세어서, 거기 없는
// 꼴을 새로 들일 때만 문다. 접미형을 쓰는 계약에서는 아무 말도 하지 않는다.
//
// 관문이 좋은 답을 막은 일이 이 세션에만 두 번 있었다(#167 enum, #181 꾸러미
// 붙은 enum). 그래서 셋을 지킨다.
//
//   - **HEAD 에서 센다.** 작업 트리에는 이번 수정이 이미 들어 있어, 새 이름이
//     제 표로 관행을 만든다(#166 에서 겪은 그것).
//   - **표본이 적으면 말하지 않는다.** 한둘을 보고 단정하면 멀쩡한 수정을 문다.
//   - **압도적일 때만 문다.** 접두형이 접미형의 다섯 배를 넘어야 한다.
const (
	minNameSamples = 20
	nameDominance  = 5
)

func suffixNameOffHabit(repoPath, diff string) []guard.Violation {
	if repoPath == "" {
		return nil
	}
	// 이번에 새로 만든 메시지 이름.
	added := map[string]bool{}
	for _, line := range strings.Split(diff, "\n") {
		if !strings.HasPrefix(line, "+") || strings.HasPrefix(line, "+++") {
			continue
		}
		if m := reMessageOpen.FindStringSubmatch(line[1:]); m != nil {
			added[m[1]] = true
		}
	}
	if len(added) == 0 {
		return nil
	}

	pre, suf := 0, 0
	forEachProto(repoPath, func(rel, _ string) {
		for _, line := range strings.Split(protoAtHead(repoPath, rel), "\n") {
			m := reMessageOpen.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			switch {
			case strings.HasPrefix(m[1], "Request"), strings.HasPrefix(m[1], "Response"):
				pre++
			case strings.HasSuffix(m[1], "Request"), strings.HasSuffix(m[1], "Response"):
				suf++
			}
		}
	})
	if pre < minNameSamples || pre <= suf*nameDominance {
		return nil
	}

	var bad []string
	for name := range added {
		if strings.HasPrefix(name, "Request") || strings.HasPrefix(name, "Response") {
			continue
		}
		if strings.HasSuffix(name, "Request") || strings.HasSuffix(name, "Response") {
			bad = append(bad, name)
		}
	}
	if len(bad) == 0 {
		return nil
	}
	sort.Strings(bad)

	var fix []string
	for _, n := range bad {
		fix = append(fix, n+" → "+flipToPrefix(n))
	}
	return []guard.Violation{{
		Why: fmt.Sprintf("%s 를 접미형으로 만들었다 — 이 계약은 접두형이다", strings.Join(bad, ", ")),
		Evidence: []string{
			fmt.Sprintf("이 계약이 실제로 쓰는 꼴: RequestXxx·ResponseXxx %d개 · 접미형 %d개", pre, suf),
			"이렇게 고친다: " + strings.Join(fix, " · "),
			"이름이 꼴을 벗어나면 쓰는 쪽이 찾지 못하고, 다음 사람이 어느 쪽을 따라야 할지 모른다.",
		},
	}}
}

// flipToPrefix 는 접미형 이름을 접두형으로 뒤집어 보여 준다.
// 「고쳐라」 라고만 하지 않고 **고친 모양**을 준다.
func flipToPrefix(name string) string {
	for _, tail := range []string{"Request", "Response"} {
		if strings.HasSuffix(name, tail) {
			return tail + strings.TrimSuffix(name, tail)
		}
	}
	return name
}
