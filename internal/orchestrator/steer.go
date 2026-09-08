package orchestrator

import (
	"strings"
)

// 도는 도중에 사람이 보낸 지시에서 **계산할 수 있는 것**만 꺼낸다.
//
// 자유롭게 쓴 글을 이미 세운 계획에 그대로 반영할 수는 없다. 그래도 셀 수
// 있는 것이 셋 있다 — 멈춰라, 이 저장소만, 이 저장소는 빼라. 그것만 코드로
// 처리하고 나머지는 **그대로 사람에게 되돌린다**(PR 본문과 알림에 싣는다).
// 삼키는 것이 가장 나쁘다 — 사람은 말했다고 여긴다.

// SteerAction 은 지시에서 읽어낸 것이다.
type SteerAction struct {
	Stop    bool     // 멈춰라
	Only    []string // 이 저장소만
	Exclude []string // 이 저장소는 빼라
	Note    string   // 코드로 처리하지 못한 말. 그대로 사람에게 되돌린다
}

var stopWords = []string{"멈춰", "멈추", "중지", "그만", "취소", "stop", "cancel", "abort"}

// 저장소 이름 뒤에 이런 말이 붙으면 빼라는 뜻이다.
var excludeWords = []string{"빼", "제외", "말고", "않고", "except", "exclude", "without"}

// 이런 말이 붙으면 그것만 하라는 뜻이다.
var onlyWords = []string{"만", "only", "just"}

// ParseSteer 는 지시를 읽는다. repos 는 이 작업이 다루는 저장소 이름들이다.
func ParseSteer(msg string, repos []string) SteerAction {
	low := strings.ToLower(msg)
	act := SteerAction{Note: strings.TrimSpace(msg)}

	for _, w := range stopWords {
		if strings.Contains(low, w) {
			act.Stop = true
		}
	}

	// 저장소 이름이 나오면 그 이름 뒤의 말로 "만" 인지 "빼라" 인지 가른다.
	for _, r := range repos {
		i := strings.Index(low, strings.ToLower(r))
		if i < 0 {
			continue
		}
		tail := low[i+len(r):]
		if cut := 24; len(tail) > cut {
			tail = tail[:cut]
		}
		switch {
		case containsAny(tail, excludeWords):
			act.Exclude = appendOnceStr(act.Exclude, r)
		case containsAny(tail, onlyWords):
			act.Only = appendOnceStr(act.Only, r)
		default:
			// 이름만 말했으면 "그것만" 으로 읽는다. 빼라는 말은 대개 붙는다.
			act.Only = appendOnceStr(act.Only, r)
		}
	}
	// 빼라고 한 것은 "그것만" 이 아니다.
	for _, e := range act.Exclude {
		act.Only = withoutStr(act.Only, e)
	}
	return act
}

func withoutStr(xs []string, x string) []string {
	out := make([]string, 0, len(xs))
	for _, v := range xs {
		if v != x {
			out = append(out, v)
		}
	}
	return out
}

func containsAny(s string, words []string) bool {
	for _, w := range words {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

// KeepSteered 는 지시에 맞게 남은 계획을 걸러 준다.
func KeepSteered(repos []string, act SteerAction) []string {
	var out []string
	for _, r := range repos {
		if len(act.Only) > 0 && !hasRepo(act.Only, r) {
			continue
		}
		if hasRepo(act.Exclude, r) {
			continue
		}
		out = append(out, r)
	}
	return out
}
