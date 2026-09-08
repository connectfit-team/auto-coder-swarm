package orchestrator

import (
	"os"
	"strconv"
)

// 한 요청이 저장소 여럿에 걸치면 저장소마다 작업이 하나씩 필요하다.
//
// 연쇄(chain reaction) 기계는 이미 있었다 — 고친 diff 로 영향을 분석해
// 다른 저장소의 작업을 만들고, 순환을 막고, 확신도 0.7 아래는 버린다.
// 그런데 **깊이가 0 이면 그 기계는 아예 돌지 않는다.** 화면(cms)이 요청을
// 넣을 때 깊이를 안 보내므로 언제나 0 이었다. 그래서 결함 흐름은 늘 한
// 저장소만 고쳤고, 화면에는 "PR 열기" 단추가 하나만 떴다.
//
// "고용주웹에서 연결보류 기능을 추가" 는 ceo·worker·protogen 에 걸친다.
// 깊이 0 으로는 답이 될 수 없는 요청이다.
//
// 고리는 세 겹으로 묶여 있다 — 깊이가 줄어들고, 이미 지난 저장소는 막히고,
// 확신도가 낮으면 버린다. 무한히 퍼지지 않는다.

const defaultChainDepth = 1

// chainDepth 는 요청이 안 적었을 때 쓸 깊이다.
//
// SWARM_CHAIN_DEPTH 로 바꿀 수 있다. 0 으로 두면 연쇄를 끈다 — 한 저장소만
// 고치던 옛 동작으로 돌아간다.
func chainDepth(reqDepth int) int {
	if reqDepth > 0 {
		return reqDepth
	}
	if v := os.Getenv("SWARM_CHAIN_DEPTH"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
	}
	return defaultChainDepth
}
