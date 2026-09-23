package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// 예산 끝까지 채운 프롬프트를 실제 서버가 받는지 본다.
//
// 이 시험이 두 가지를 잡았다. 어림만으로 묶었을 때 빽빽한 코드에서 4% 모자랐고
// (한계에서 25토큰 남았다), 지난 문맥 몫을 글자 수로 빼 한글에서 예산을 넘겼다.
// 서버가 없으면 건너뛴다.
func TestBudgetEdgeAcceptedByServer(t *testing.T) {
	kinds := map[string]string{
		"code":   "export const value = { id: 1, name: 'x' }\n",
		"korean": "// 이 줄은 한글로만 되어 있고 문장부호도 섞여 있다 — 「보기」…\n",
		"mixed":  "func handle(ctx context.Context) error { // 연결 보류 상태를 담는다\n",
	}
	for kind, unit := range kinds {
		var b strings.Builder
		for b.Len() < 200000 {
			b.WriteString(unit)
		}
		body, cut := ClipToTokens(b.String(), InputTokenBudget())
		if !cut {
			t.Fatalf("%s: 안 잘렸다", kind)
		}
		payload, _ := json.Marshal(map[string]any{
			"model":      modelName(),
			"messages":   []map[string]string{{"role": "user", "content": body}},
			"max_tokens": outputReserve(),
		})
		cl := &http.Client{Timeout: 300 * time.Second}
		resp, err := cl.Post(strings.TrimSuffix(tokenizeEndpoint(), "/tokenize")+"/v1/chat/completions",
			"application/json", bytes.NewReader(payload))
		if err != nil {
			t.Skipf("서버에 못 붙었다: %v", err)
		}
		var out struct {
			Usage struct {
				PromptTokens int `json:"prompt_tokens"`
			} `json:"usage"`
		}
		json.NewDecoder(resp.Body).Decode(&out)
		resp.Body.Close()

		used := out.Usage.PromptTokens
		fmt.Printf("%-7s 센 값 %5d → 실제 %5d 토큰 · 출력 몫 %d · 창 %d · HTTP %d\n",
			kind, CountTokens(body), used, outputReserve(), modelWindow(), resp.StatusCode)
		if resp.StatusCode != 200 {
			t.Fatalf("%s: 예산 끝에서 %d 가 났다", kind, resp.StatusCode)
		}
		if used+outputReserve() > modelWindow() {
			t.Fatalf("%s: 창을 넘었다 (%d + %d > %d)", kind, used, outputReserve(), modelWindow())
		}
	}
}
