package agent

import (
	"fmt"
	"strings"
	"testing"
)

func TestBudgetDerivedFromWindow(t *testing.T) {
	if got, want := InputTokenBudget(), 16384-4096-512; got != want {
		t.Fatalf("입력 예산 %d, 기대 %d", got, want)
	}
	t.Setenv("LLM_MAX_MODEL_LEN", "32768")
	t.Setenv("LLM_MAX_TOKENS", "2048")
	if got, want := InputTokenBudget(), 32768-2048-512; got != want {
		t.Fatalf("환경을 안 읽는다: %d, 기대 %d", got, want)
	}
	t.Setenv("LLM_MAX_MODEL_LEN", "1000")
	if InputTokenBudget() < 1024 {
		t.Fatal("바닥이 없다 — 음수 예산이 나오면 아무것도 못 보여 준다")
	}
}

// 적게 세면 400 이 나고 그 시도가 통째로 날아간다. 많이 세는 쪽으로 틀려야 한다.
func TestEstimateIsConservative(t *testing.T) {
	// 실측: 아래 줄 800개가 9,654 토큰이었다.
	line := "// 한 줄의 주석이다 abcdefgh\n"
	got := EstimateTokens(strings.Repeat(line, 800))
	if got < 9654 {
		t.Fatalf("적게 셌다: %d < 실측 9654", got)
	}
	if got > 9654*2 {
		t.Fatalf("너무 많이 셌다: %d — 쓸 수 있는 자리를 버린다", got)
	}
}

func TestClipRespectsBudget(t *testing.T) {
	src := strings.Repeat("func f() { return 1 }\n", 2000)
	got, cut := ClipToTokens(src, 500)
	if !cut {
		t.Fatal("잘랐다고 알리지 않았다")
	}
	if n := CountTokens(got); n > 500 {
		t.Fatalf("예산을 넘겼다: %d 토큰", n)
	}
	if CountTokens(got) < 400 {
		t.Fatalf("너무 많이 버렸다: %d 토큰 (예산 500)", CountTokens(got))
	}
	if _, cut := ClipToTokens("짧다", 500); cut {
		t.Fatal("예산에 드는데 잘랐다")
	}
}

func TestLinesForBudgetScales(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 500; i++ {
		fmt.Fprintf(&b, "\tconst value%d = %d\n", i, i)
	}
	src := b.String()
	narrow := linesForBudget(src, 1000)
	wide := linesForBudget(src, 11776)
	if narrow >= wide {
		t.Fatalf("예산이 커져도 줄 수가 안 늘었다: %d → %d", narrow, wide)
	}
	// 어림이 맞는지 — 그만큼 줄을 담으면 예산 안이어야 한다.
	lines := strings.SplitAfter(src, "\n")
	if wide < len(lines) {
		if n := EstimateTokens(strings.Join(lines[:wide], "")); n > 11776 {
			t.Fatalf("어림이 낙관적이다: %d줄이 %d 토큰 (예산 11776)", wide, n)
		}
	}
}

// 큰 파일을 넣어도 프롬프트 전체가 예산 안이어야 한다.
func TestPromptStaysWithinBudget(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 3000; i++ {
		fmt.Fprintf(&b, "export const thing%d = { id: %d, name: '이름 %d' }\n", i, i, i)
	}
	src := b.String()
	instr := "thing42 의 name 을 고쳐라"

	outline := FileOutline("big.ts", src)
	rest := outline + instr + editBlockRules
	room := InputTokenBudget() - EstimateTokens(rest) - pastContextReserve()

	shown := src
	if r := relevantRegionsWithin(src, instr, linesForBudget(src, room)); r != "" {
		shown = r
	}
	shown, _ = ClipToTokens(shown, room)

	total := EstimateTokens(rest + shown + strings.Repeat("가", pastContextBudget()))
	if total > InputTokenBudget() {
		t.Fatalf("프롬프트가 예산을 넘는다: %d > %d", total, InputTokenBudget())
	}
	if EstimateTokens(shown) < 3000 {
		t.Fatalf("예산이 남는데 적게 보여 준다: %d 토큰 (여유 %d)", EstimateTokens(shown), room)
	}
}
