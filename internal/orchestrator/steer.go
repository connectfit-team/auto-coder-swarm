package orchestrator

import (
	"strings"
	"unicode/utf8"
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

// 멈추라는 말을 뒤집는 말. "중지하지 말고 계속해" 는 멈추라는 뜻이 아니다.
var negations = []string{
	"말고", "말아", "마라", "말것", "않", "마세요", "지 마", "지마", "지말",
	"not ", "don't", "no ",
}

// 저장소 이름 뒤에 이런 말이 붙으면 빼라는 뜻이다.
var excludeWords = []string{"빼", "제외", "말고", "않고", "except", "exclude", "without"}

// 이런 말이 붙으면 그것만 하라는 뜻이다.
var onlyWords = []string{"만", "only", "just"}

// 저장소 이름 뒤로 이만큼까지 보고 뜻을 가른다.
const steerTailWindow = 24

// ParseSteer 는 지시를 읽는다. repos 는 이 작업이 다루는 저장소 이름들이다.
//
// **읽히지 않으면 아무것도 하지 않는다.** 반만 읽고 계획을 바꾸는 것이
// 가만히 있는 것보다 나쁘다 — 사람은 말한 대로 됐다고 여긴다.
func ParseSteer(msg string, repos []string) SteerAction {
	low := strings.ToLower(msg)
	act := SteerAction{Note: strings.TrimSpace(msg)}

	for _, w := range stopWords {
		i := strings.Index(low, w)
		if i < 0 {
			continue
		}
		// "중지하지 말고" 처럼 뒤에서 뒤집는 말이 오면 멈추는 것이 아니다.
		if containsAny(tailAfter(low, i+len(w)), negations) {
			continue
		}
		act.Stop = true
	}

	for _, r := range repos {
		i := strings.Index(low, strings.ToLower(r))
		if i < 0 {
			continue
		}
		tail := tailAfter(low, i+len(r))
		switch {
		case containsAny(tail, excludeWords):
			act.Exclude = appendOnceStr(act.Exclude, r)
		case containsAny(tail, onlyWords):
			act.Only = appendOnceStr(act.Only, r)
		}
		// 이름만 말한 것은 뜻이 하나가 아니다 — "worker 에서 깨질 것 같다" 는
		// worker 만 하라는 말이 아니다. 가리는 말이 없으면 계획을 안 건드리고
		// 그 말을 사람에게 되돌린다.
	}
	for _, e := range act.Exclude {
		act.Only = withoutStr(act.Only, e)
	}
	return act
}

// tailAfter 는 그 자리 뒤의 짧은 꼬리를 준다. 룬 경계를 지킨다.
func tailAfter(s string, from int) string {
	if from > len(s) {
		return ""
	}
	tail := s[from:]
	if len(tail) > steerTailWindow {
		tail = tail[:steerTailWindow]
		// 한글은 세 바이트다. 자르다 깨지면 뒤에서 한 바이트씩 줄인다.
		for len(tail) > 0 && !utf8.ValidString(tail) {
			tail = tail[:len(tail)-1]
		}
	}
	return tail
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

func withoutStr(xs []string, x string) []string {
	out := make([]string, 0, len(xs))
	for _, v := range xs {
		if v != x {
			out = append(out, v)
		}
	}
	return out
}
