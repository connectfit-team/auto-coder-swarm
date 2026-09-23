package orchestrator

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/guard"
)

var (
	// 상태를 담는 필드의 이름. 뒤에 붙는 꼴만 본다 — `status_message` 처럼
	// 상태가 아닌 것까지 물면 안 된다.
	//
	// **`type` 은 넣지 않는다.** 실제로 세어 보니 이 계약의 `*_type` 에는
	// `string content_type`(MIME)이 일곱 곳 있다 — 글자가 맞는 자리다.
	// 그것까지 한 무리로 묶으면 string 이 습관에 들어와 검사가 조용히 꺼진다.
	// `*_status`·`*_state` 는 세 곳 모두 int32 다.
	reStateFieldName = regexp.MustCompile(`(?i)(?:^|_)(status|state)$`)
	reTypedField     = regexp.MustCompile(`^\s*(?:repeated\s+|optional\s+)?([\w.]+)\s+(\w+)\s*=\s*(\d+)\s*;`)
)

// stateFieldTypeOffHabit 는 상태 필드를 이 계약이 안 쓰는 타입으로 넣은 것을
// 잡는다.
//
// 실측 W-13896 이 `string pending_status = 15;` 를 넣었다. 그런데 이 계약의
// 상태·유형 필드는 **하나도 빠짐없이** int32 이거나 제대로 된 enum 이다
// (`int32 status`, `int32 job_type`, `CEOPolicyType policy_type` …).
// string 으로 두면 무엇이든 담을 수 있어 값이 굳지 않는다 — 부르는 쪽마다
// 다른 글자를 넣고, 서버는 그것을 다 받는다.
//
// **규칙을 박아 두지 않는다.** 이 저장소가 실제로 쓰는 타입을 세어서, 거기
// 없는 것을 새로 들일 때만 문다. 어느 계약이 string 을 쓰고 있다면 그 계약에
// 서는 아무 말도 하지 않는다.
func stateFieldTypeOffHabit(repoPath, diff string) []guard.Violation {
	// **새로 넣은 것은 습관에서 뺀다.**
	//
	// 관문이 도는 자리에서 작업 트리에는 **이미 이번 수정이 들어가 있다.**
	// 그대로 세면 새 필드가 자기 자신을 습관으로 투표한다 — 실측 W-65042 가
	// `string status = 15` 를 넣고 그 한 표로 빠져나갔다.
	adding := map[string]int{}
	for _, line := range strings.Split(diff, "\n") {
		if !strings.HasPrefix(line, "+") || strings.HasPrefix(line, "+++") {
			continue
		}
		if t, n, ok := protoField(line[1:]); ok && reStateFieldName.MatchString(n) {
			adding[t]++
		}
	}

	habit := map[string]int{}
	forEachProto(repoPath, func(rel, src string) {
		for _, line := range strings.Split(src, "\n") {
			if t, n, ok := protoField(line); ok && reStateFieldName.MatchString(n) {
				habit[t]++
			}
		}
	})
	for t, n := range adding {
		if habit[t] -= n; habit[t] <= 0 {
			delete(habit, t)
		}
	}
	// 표본이 너무 적으면 그것은 습관이 아니다. 한두 자리를 보고 단정하면
	// 멀쩡한 수정을 물게 된다.
	total := 0
	for _, n := range habit {
		total += n
	}
	if total < minHabitSamples {
		return nil
	}

	var out []guard.Violation
	for _, line := range strings.Split(diff, "\n") {
		if !strings.HasPrefix(line, "+") || strings.HasPrefix(line, "+++") {
			continue
		}
		t, n, ok := protoField(line[1:])
		if !ok || !reStateFieldName.MatchString(n) || habit[t] > 0 {
			continue
		}
		out = append(out, guard.Violation{
			Why: fmt.Sprintf("%s 를 %s 로 넣었다 — 이 계약은 상태를 그 타입으로 담지 않는다", n, t),
			Evidence: []string{
				"이 계약이 실제로 쓰는 것: " + habitNote(habit),
				"값이 굳지 않으면 부르는 쪽마다 다른 것을 넣는다. 위 가운데서 골라 쓰거나 enum 을 만들어라.",
			},
		})
	}
	return out
}

func protoField(line string) (typ, name string, ok bool) {
	m := reTypedField.FindStringSubmatch(line)
	if m == nil {
		return "", "", false
	}
	// `message X {` 같은 줄이 걸리지 않게 — 타입 자리에 키워드가 오면 아니다.
	switch m[1] {
	case "message", "enum", "service", "rpc", "option", "import", "syntax", "package", "returns":
		return "", "", false
	}
	return m[1], m[2], true
}

// habitNote 는 많이 쓰인 차례로 타입을 적는다.
func habitNote(habit map[string]int) string {
	type kv struct {
		t string
		n int
	}
	var all []kv
	for t, n := range habit {
		all = append(all, kv{t, n})
	}
	for i := range all {
		for j := i + 1; j < len(all); j++ {
			if all[j].n > all[i].n || (all[j].n == all[i].n && all[j].t < all[i].t) {
				all[i], all[j] = all[j], all[i]
			}
		}
	}
	var parts []string
	for i, e := range all {
		if i >= 5 {
			break
		}
		parts = append(parts, fmt.Sprintf("%s(%d곳)", e.t, e.n))
	}
	return strings.Join(parts, " · ")
}

// 습관이라고 말하려면 이만큼은 있어야 한다.
const minHabitSamples = 3
