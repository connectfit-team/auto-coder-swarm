package agent

import (
	"os"
	"strconv"
	"strings"
)

// 지난 문맥이 프롬프트에서 차지해도 되는 상한.
//
// 창이 8,192 토큰이고 coder.go 는 코드를 4,500자로 줄여 그 창에 맞춘다.
// 지난 문맥은 그 앞에 붙으므로 여기서 묶지 않으면 코드 쪽 예산이 그대로
// 무너진다.
const pastContextLimit = 1200

func pastContextBudget() int {
	if v, err := strconv.Atoi(strings.TrimSpace(os.Getenv("SWARM_PAST_CONTEXT_CHARS"))); err == nil && v >= 0 {
		return v
	}
	return pastContextLimit
}

// CompactContext 는 쌓인 단계 기록에서 프롬프트에 넣을 부분만 남긴다.
//
// 기록은 한 줄이 "[시각] STAGE: 말" 이고, 한 작업이 여러 번 시도되면 같은
// 줄이 시도 횟수만큼 되풀이된다. 실제로 5KB 짜리 기록이 열 종류 남짓한
// 줄을 여덟 번 되풀이한 것이었다 — 모델에게는 같은 말을 여덟 번 하는 셈이라
// 자리만 먹는 것이 아니라 그쪽으로 끌린다.
//
// 그래서 두 가지만 한다. 한 단계는 **마지막 것만** 남기고, 그러고도 예산을
// 넘으면 **뒤에서부터** 자른다. 무엇이 중요한지는 고르지 않는다 — 고르는 일은
// 모델의 몫이고, 여기서 할 일은 지난 것을 걷어내는 것뿐이다.
//
// 말이 달라도 단계가 같으면 지운다. `STRATEGY_PARSED: 파일 0개` 뒤에
// `STRATEGY_PARSED: 파일 3개` 가 오면 앞의 것은 이미 틀린 사실인데, 둘을
// 나란히 넣으면 모델이 지난 것을 지금으로 읽는다. 지운 줄은 깊은 로그
// (AddDeepLog)에 그대로 남으므로 진단에서 잃는 것은 없다.
func CompactContext(state string, limit int) string {
	if limit <= 0 || strings.TrimSpace(state) == "" {
		return ""
	}

	lines := strings.Split(strings.TrimSpace(state), "\n")

	// 한 단계가 다시 나오면 앞의 것을 지운다 — 순서는 마지막 등장 자리를 따른다.
	lastAt := make(map[string]int, len(lines))
	for i, ln := range lines {
		lastAt[stageOf(ln)] = i
	}
	kept := make([]string, 0, len(lines))
	for i, ln := range lines {
		if lastAt[stageOf(ln)] == i {
			kept = append(kept, ln)
		}
	}

	// 뒤에서부터 예산만큼 담는다. 최근 것이 지금 판단에 가깝다.
	total := 0
	cut := len(kept)
	for i := len(kept) - 1; i >= 0; i-- {
		total += len(kept[i]) + 1
		if total > limit {
			cut = i + 1
			break
		}
		cut = i
	}
	return strings.Join(kept[cut:], "\n")
}

// stageOf 는 "[15:04:05] STAGE: 말" 에서 STAGE 를 뽑는다. 그 모양이 아니면
// 줄 전체를 열쇠로 쓴다 — 묶을 근거가 없으면 묶지 않는다.
func stageOf(line string) string {
	rest := line
	if strings.HasPrefix(rest, "[") {
		if i := strings.Index(rest, "] "); i > 0 && i <= 9 {
			rest = rest[i+2:]
		}
	}
	if i := strings.Index(rest, ": "); i > 0 && !strings.Contains(rest[:i], " ") {
		return rest[:i]
	}
	return line
}
