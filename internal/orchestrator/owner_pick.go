package orchestrator

import (
	"fmt"
	"strings"
)

// 점수가 이만큼 벌어져야 고른다. 한 점 차이는 흔들림이다.
const scoreMargin = 2

// pickContractAmong 은 닿은 계약이 여럿일 때 하나를 고른다.
//
// 기계가 후보와 근거(파일 → 공장 → 계약)와 그 계약에 적힌 말을 이미 다 뽑아
// 놓았다. 남은 것은 고르는 일 하나뿐이고, 그것을 **한 번에 하나씩 점수로**
// 묻는다. 물음이 크면 모델이 답을 못 낸다.
func (t *taskContext) pickContractAmong(order []string, evidence map[string]string, missing []string) (picked, why string, rejected bool) {
	scores := t.scoreContracts(order, evidence, missing)
	best, top, second, read := bestByScore(order, scores)

	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CONTRACT_SCORES",
		fmt.Sprintf("후보 %d 에 점수를 매겼다 — 가장 높은 것 %d점, 다음 %d점", len(order), top, second),
		scoreNote(order, scores), best)

	switch {
	case !read:
		return "", fmt.Sprintf("계약 후보 %d 를 두고 점수를 한 번도 못 읽었다", len(order)), false
	case top <= 0:
		// **고르지 못한 것과 다르다.** 어느 것도 알맞지 않다고 답한 것이다.
		return "", fmt.Sprintf("계약 후보 %d 가 모두 0점이다 — 이 가운데 없다", len(order)), true
	case best == "":
		// **줄 세우지 못하면 둘씩 견준다.** 열여덟을 줄 세우는 것과 둘 중
		// 하나를 고르는 것은 다른 물음이다(tournament.go 의 실측 참고).
		if p, why := t.tournamentPick(order, evidence, missing); p != "" {
			return p, fmt.Sprintf("%d점이 둘 이상이라 %s", top, why), false
		}
		return "", fmt.Sprintf("계약 후보 %d 가운데 %d점이 둘 이상이다 — 차례가 갈리지 않는다", len(order), top), false
	case top-second < scoreMargin:
		// **엇비슷하면 고르지 않는다.**
		//
		// 이 모델은 계약을 줄 세우지 못한다. 재어 보면 경로만 주었을 때
		// 다섯 계약이 모두 3점이었고, 근거 글을 붙이면 엉뚱한 계약이 정답보다
		// 높게 나왔다(정답 2·2·2 / 오답 3·2·3). 갈리는 것처럼 보인 회차는
		// 계약에 적힌 「쓰지 마라」 라는 낱말에 반응한 것이지 판단이 아니다.
		//
		// 한 점 차이로 고르면 그 흔들림이 그대로 답이 된다. 뚜렷할 때만
		// 쓰고, 아니면 따라간 것과 타입을 묻는 길에 맡긴다 — 그쪽이 파일에
		// 적힌 사실이다.
		if p, why := t.tournamentPick(order, evidence, missing); p != "" {
			return p, fmt.Sprintf("%d점과 %d점이라 뚜렷하지 않아 %s", top, second, why), false
		}
		return "", fmt.Sprintf("계약 후보 %d 가운데 가장 높은 것이 %d점, 다음이 %d점이다 — 뚜렷하지 않다",
			len(order), top, second), false
	}
	return best, fmt.Sprintf("계약 후보 %d 가운데 %s 가 %d점으로 가장 높다(다음은 %d점)",
		len(order), best, top, second), false
}

// scoreNote 는 점수를 높은 차례로 적는다. 왜 그것이 뽑혔는지 사람이 본다.
func scoreNote(order []string, scores map[string]int) string {
	type row struct {
		k string
		s int
	}
	rows := make([]row, 0, len(order))
	for _, k := range order {
		rows = append(rows, row{k, scores[k]})
	}
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0 && rows[j].s > rows[j-1].s; j-- {
			rows[j], rows[j-1] = rows[j-1], rows[j]
		}
	}
	var out []string
	for i, r := range rows {
		if i >= 6 {
			out = append(out, fmt.Sprintf("… 그 밖 %d개", len(rows)-i))
			break
		}
		out = append(out, fmt.Sprintf("%d점 %s", r.s, r.k))
	}
	return strings.Join(out, " · ")
}
