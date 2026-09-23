package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
)

// 줄 세우지 못하면 **둘씩 견준다.**
//
// 후보 18개에 점수를 매기게 했더니 **전부 2점**이었다(실측 W-42583). 그 뒤는
// 모델에게 타입을 묻는 길이고, 거기서 엉뚱한 계약이 나왔다.
//
// 같은 모델에 둘만 주고 물으면 답한다. 여덟 쌍을 자리까지 바꿔 재 보니
// **8/8 만장일치**였다(connect vs ceo·notif·purchase·manager·worker, 양쪽 순서).
//
// 열여덟을 줄 세우는 것과 둘 중 하나를 고르는 것은 다른 물음이다. 작업을
// 쪼개면 되는 일을, 못 한다고 접을 까닭이 없다.
//
// **근거 글은 붙이지 않는다 — 경로만 준다.**
//
// 점수 쪽에서 이미 알려진 병이 여기에도 그대로 있다. 근거(따라간 경로·계약에
// 적힌 말)를 붙여 재 보니 4쌍 가운데 2쌍이 틀렸고, 살펴보니 관련성이 아니라
// **덧붙은 말이 있느냐·긴가**에 반응했다 — 경고가 적힌 계약을 골랐다.
//
//	근거 붙임: connect vs ceo → A(맞음) · ceo vs connect → A(틀림)
//	           connect vs notif → B(틀림) · worker vs connect → B(맞음)
//	경로만   : 여덟 쌍 전부 맞음
//
// 경고를 못 보고 고르는 것 아니냐 — 경고는 점수 쪽이 본다. 토너먼트는 점수가
// 갈리지 못했을 때만 열리고, 경로만으로도 경고가 붙은 계약(notif)을 양쪽
// 순서에서 다 물리쳤다.

const (
	// 한 판을 몇 번 묻는가. 한 번은 자리를 바꿔 물어 위치에 쏠렸는지 본다.
	matchVotes = 3
	// 토너먼트에 올릴 후보의 상한. 판 수는 후보 수보다 하나 적다.
	maxTournamentCandidates = 32
)

// tournamentPick 은 둘씩 견주어 하나를 남긴다. 고르지 못하면 빈 문자열이다.
func (t *taskContext) tournamentPick(order []string, missing []string) (string, string) {
	field := order
	if len(field) > maxTournamentCandidates {
		field = field[:maxTournamentCandidates]
	}
	switch len(field) {
	case 0:
		return "", ""
	case 1:
		return field[0], "후보가 하나뿐이다"
	}

	matches, ties := 0, 0
	for len(field) > 1 {
		var next []string
		for i := 0; i < len(field); i += 2 {
			if i+1 >= len(field) {
				next = append(next, field[i]) // 홀수면 부전승
				continue
			}
			a, b := field[i], field[i+1]
			matches++
			win, decided := t.playMatch(a, b, missing)
			if !decided {
				ties++
				win = a // 갈리지 않으면 앞의 것을 남긴다 — 차례가 정해져 있어야 답이 안 흔들린다
			}
			next = append(next, win)
		}
		field = next
	}

	why := fmt.Sprintf("둘씩 견주어 골랐다 — 후보 %d개, 판 %d번", len(order), matches)
	if ties > 0 {
		why += fmt.Sprintf(", 그 가운데 %d번은 갈리지 않아 앞의 것을 남겼다", ties)
	}
	return field[0], why
}

// playMatch 는 한 판을 여러 번 물어 이긴 쪽을 준다.
//
// 한 번은 **자리를 바꿔** 묻는다. 늘 앞의 것을 고르는 것인지 아닌지가
// 그렇게만 드러난다.
func (t *taskContext) playMatch(a, b string, missing []string) (string, bool) {
	votes := map[string]int{}
	for i := 0; i < matchVotes; i++ {
		x, y := a, b
		swapped := i == matchVotes-1
		if swapped {
			x, y = b, a
		}
		pick, ok := t.askMatch(x, y, missing)
		if !ok {
			continue
		}
		if pick == "A" {
			votes[x]++
		} else {
			votes[y]++
		}
	}
	switch {
	case votes[a] > votes[b]:
		return a, true
	case votes[b] > votes[a]:
		return b, true
	}
	return "", false
}

// askMatch 는 A·B 한 글자를 받는다.
func (t *taskContext) askMatch(a, b string, missing []string) (string, bool) {
	// 근거 글을 붙이지 않는다(위 설명 참고). 경로만 준다.
	prompt := fmt.Sprintf(`[없어서 못 만드는 것]
%s

[계약 A]
%s

[계약 B]
%s

둘 중 어느 계약에 위 상태를 담아야 하는가? A 또는 B 한 글자만 적어라.
다른 말은 쓰지 마라.`,
		strings.Join(missing, "\n"), a, b)

	ctx, cancel := context.WithTimeout(t.ctx, stateCheckTimeout)
	defer cancel()
	raw, err := agent.CallLLM(ctx, t.primaryLLM, "ContractMatch", prompt)
	if err != nil {
		return "", false
	}
	return parseAB(raw)
}

// parseAB 는 답에서 A 인지 B 인지만 뽑는다.
//
// 「다른 말은 쓰지 마라」 고 해도 붙여 온다. **낱말 경계로 갈라야 한다** —
// A·B 글자만 남기고 가르면 "contract Alpha" 에서 A 가 홀로 선 것처럼 보인다.
func parseAB(raw string) (string, bool) {
	for _, f := range strings.FieldsFunc(raw, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		switch strings.ToUpper(f) {
		case "A":
			return "A", true
		case "B":
			return "B", true
		}
	}
	return "", false
}
