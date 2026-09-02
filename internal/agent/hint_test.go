package agent

import (
	"context"
	"strings"
	"testing"
)

func TestRetryTellsWhatWasWrong(t *testing.T) {
	type out struct {
		Repo string `json:"repo_name"`
	}
	var got out
	var prompts []string

	// 첫 답은 산문, 둘째는 제대로 된 JSON.
	n := 0
	call := func(p string) (string, error) {
		prompts = append(prompts, p)
		n++
		if n == 1 {
			return "Repo 는 cms 입니다. 아래에 계획을 적겠습니다.", nil
		}
		return `{"repo_name":"cms"}`, nil
	}

	if _, err := callJSONWith(context.Background(), "계획을 내라", &got, "Planner", call); err != nil {
		t.Fatalf("두 번째에 성공해야 한다: %v", err)
	}
	if got.Repo != "cms" {
		t.Fatalf("값을 못 읽었다: %+v", got)
	}
	if len(prompts) != 2 {
		t.Fatalf("두 번 물어야 한다: %d", len(prompts))
	}

	// 두 번째 프롬프트에 무엇이 틀렸는지 담겨야 한다.
	second := prompts[1]
	for _, want := range []string{"방금 낸 답을 못 읽었다", "네가 낸 답의 앞부분", "Repo 는 cms 입니다"} {
		if !strings.Contains(second, want) {
			t.Errorf("재시도가 %q 를 안 알려 준다", want)
		}
	}
	if !strings.Contains(second, "첫 글자가 R 다") {
		t.Error("첫 글자를 짚어 주지 않는다")
	}
}

func TestParseHintQuietWhenNoError(t *testing.T) {
	if parseHint(nil, "무엇이든") != "" {
		t.Error("오류가 없으면 아무 말도 하지 않아야 한다")
	}
}
