package agent

import (
	"os"
	"strconv"
	"strings"
)

// 창 예산.
//
// 오래 「창이 8,192 토큰」 이라고 적혀 있었다. 근거는 입력 13,997 토큰에서
// 400 이 난 실측이었는데, 그 오류는 창이 8,192 라서가 아니라
// `13,997 + 4,096(출력 몫) > 16,384` 여서 났다. 서버는 실제로
// `--max-model-len 16384` 로 떠 있고, 9,654 토큰짜리 입력은 멀쩡히 통과한다.
//
// 그래서 숫자를 손으로 적지 않는다 — 창에서 출력 몫과 여유를 빼 역산한다.
const (
	defaultModelWindow = 16384
	defaultOutputCap   = 4096

	// 토큰을 세는 법이 우리와 서버가 다르다. 적게 세면 400 이 나므로 여유를 둔다.
	windowSafety = 512
)

func modelWindow() int {
	if n, err := strconv.Atoi(strings.TrimSpace(os.Getenv("LLM_MAX_MODEL_LEN"))); err == nil && n > 0 {
		return n
	}
	return defaultModelWindow
}

func outputReserve() int {
	if n, err := strconv.Atoi(strings.TrimSpace(os.Getenv("LLM_MAX_TOKENS"))); err == nil && n > 0 {
		return n
	}
	return defaultOutputCap
}

// InputTokenBudget 는 프롬프트 전체가 쓸 수 있는 토큰이다.
func InputTokenBudget() int {
	n := modelWindow() - outputReserve() - windowSafety
	if n < 1024 {
		return 1024
	}
	return n
}

// EstimateTokens 는 서버에 못 물었을 때 쓰는 어림이다(CountTokens 참고).
//
// **넉넉히** 센다. 적게 세면 요청이 400 으로 떨어지고 그 시도가 통째로
// 날아가므로, 틀리더라도 많이 세는 쪽으로 틀린다. 실측으로 가장 빡빡했던 것은
// 빽빽한 코드의 2.62자/토큰이었다 — 2.5 로 나눈다.
func EstimateTokens(s string) int {
	ascii, other := 0, 0
	for _, r := range s {
		if r < 128 {
			ascii++
		} else {
			other++
		}
	}
	return (ascii*2/5+other)*11/10 + 1
}

// ClipToTokens 는 앞에서부터 예산만큼만 남긴다.
//
// 어림으로 자리를 좁힌 뒤 **실제로 세어** 맞춘다. 어림만으로 묶으면 두 쪽으로
// 틀린다 — 빽빽한 코드에서는 4% 모자라 경계에서 400 이 나고, 한글에서는 너무
// 많이 잡아 예산의 40%를 남긴 채 버린다.
func ClipToTokens(s string, maxTokens int) (string, bool) {
	if maxTokens <= 0 {
		return "", true
	}
	if CountTokens(s) <= maxTokens {
		return s, false
	}

	r := []rune(s)

	// 1단계 — 어림으로 확실히 예산 안인 자리를 찾는다. 여기는 통신이 없다.
	lo, hi := 0, len(r)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if EstimateTokens(string(r[:mid])) <= maxTokens {
			lo = mid
		} else {
			hi = mid - 1
		}
	}

	// 2단계 — 실제로 세며 예산 끝까지 늘린다. 통신은 여덟 번 안쪽(한 번 14ms).
	hi = len(r)
	for i := 0; i < 8 && lo < hi; i++ {
		mid := (lo + hi + 1) / 2
		if CountTokens(string(r[:mid])) <= maxTokens {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	// 어림이 낙관적이었으면 1단계 자리도 예산을 넘는다. 실제로 세어 보고 줄인다.
	for i := 0; i < 8 && lo > 0 && CountTokens(string(r[:lo])) > maxTokens; i++ {
		lo = lo * 95 / 100
	}
	return string(r[:lo]), true
}

// linesForBudget 는 그 예산에 들어갈 만한 줄 수를 어림한다. 줄 하나의 무게는
// 파일마다 다르므로 그 파일에서 직접 잰다.
func linesForBudget(src string, tokens int) int {
	lines := strings.Count(src, "\n") + 1
	per := EstimateTokens(src)/lines + 1
	return tokens / per
}

// pastContextReserve 는 CallLLM 이 프롬프트 앞에 붙이는 지난 문맥의 몫이다.
//
// 그 몫은 **글자 수**로 묶인다(pastContextBudget). 전부 한글이면 글자 하나가
// 1토큰을 넘으므로, 글자 수를 그대로 빼면 그만큼 예산을 넘긴다.
func pastContextReserve() int {
	return EstimateTokens(strings.Repeat("\uac00", pastContextBudget()))
}

// 절차·뼈대·지시문이 예산을 다 먹어도 코드는 이만큼은 보여 준다.
const minShownTokens = 1500
