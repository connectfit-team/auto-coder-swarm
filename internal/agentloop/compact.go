package agentloop

import (
	"context"
	"strings"

	"google.golang.org/genai"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
)

// 요약에 보낼 글의 상한. 접을 것이 아무리 많아도 이만큼만 보낸다.
const compactInputTokens = 2000

// 요약이 차지할 수 있는 토큰. 이보다 길면 요약이 아니다.
const compactOutputTokens = 400

// historyBudget 은 궤적이 쓸 수 있는 토큰이다.
//
// 창 전부를 쓰지 않는다 — 이번 물음과 이번에 받을 도구 결과가 들어갈 자리를
// 남겨야 한다. CIE 가 같은 자리에 3/4 를 쓴다.
func historyBudget() int { return agent.InputTokenBudget() * 3 / 4 }

// compact 는 창이 차면 오래된 차례를 **요약해서** 접는다.
//
// 그냥 버리면 본 것이 사라져 같은 도구를 또 부른다. codex 는 compact*.rs 에서
// 같은 일을 하고, CIE 의 manageContext 는 버리기만 한다 — 거기서 한 걸음 더 간다.
//
// 요약이 실패해도 루프를 멈추지 않는다. 그때는 전처럼 앞에서 자른다.
func (l *Loop) compact(ctx context.Context, history []*genai.Content) []*genai.Content {
	budget := historyBudget()
	if len(history) < 5 || tokensOf(history) <= budget {
		return fit(history, budget, 1)
	}

	// 맨 앞(지시·도구·목표)과 **최근 두 짝**은 남긴다.
	head, tail := history[:1], history[len(history)-4:]
	middle := history[1 : len(history)-4]
	if len(middle) == 0 {
		return fit(history, budget, 1)
	}

	note, err := l.summarize(ctx, middle)
	if err != nil || strings.TrimSpace(note) == "" {
		l.log("COMPACT_FAILED", "요약하지 못했다 — 앞에서 자른다")
		return fit(history, budget, 1)
	}
	l.log("COMPACT", "오래된 차례 "+itoa(len(middle))+"개를 요약으로 접었다")

	out := append([]*genai.Content{}, head...)
	out = append(out, userTurn("[앞선 조사 요약 — 이미 한 것이다. 되풀이하지 마라]\n"+note))
	out = append(out, tail...)
	// 맨 앞과 요약은 지킨다.
	return fit(out, budget, 2)
}

func (l *Loop) summarize(ctx context.Context, middle []*genai.Content) (string, error) {
	if l.Summarize != nil {
		return l.Summarize(ctx, flatten(middle))
	}
	body, _ := agent.ClipToTokens(flatten(middle), compactInputTokens)
	ask := "아래는 조사 기록이다. **열 줄 안으로** 줄여라.\n" +
		"- 어떤 도구를 어떤 인자로 불렀는지\n" +
		"- 무엇을 알아냈는지 (사실만)\n" +
		"- 무엇이 실패했는지\n" +
		"새로 추측하지 마라. 도구를 부르지 마라.\n\n" + body
	text, err := l.say(ctx, []*genai.Content{userTurn(ask)})
	if err != nil {
		return "", err
	}
	out, _ := agent.ClipToTokens(strings.TrimSpace(text), compactOutputTokens)
	return out, nil
}

func flatten(cs []*genai.Content) string {
	var b strings.Builder
	for _, c := range cs {
		for _, p := range c.Parts {
			b.WriteString(p.Text)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func tokensOf(cs []*genai.Content) int {
	n := 0
	for _, c := range cs {
		for _, p := range c.Parts {
			n += agent.EstimateTokens(p.Text)
		}
	}
	return n
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
