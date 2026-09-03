package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// 판정은 한 모델에서만 나와야 한다.
//
// 큐에는 일꾼이 여럿이고 서로 다른 모델을 쓴다. 계획·비평·검토를 큐로 보내면
// 그것이 두 모델에 무작위로 나뉘어, 같은 요청에 실행마다 다른 결론이 나온다.
// code-insight-engine 에서 실측한 판정 일치율은 87% 였고, 맥을 같은 계열
// 모델로 바꿔도 그대로였다 — 양자화 차이만으로 경계선 판정이 뒤집힌다.
type DirectModel struct {
	name string
	base string
	hc   *http.Client
}

func NewDirectModel(name, baseURL string) model.LLM {
	return &DirectModel{name: name, base: strings.TrimRight(baseURL, "/"), hc: &http.Client{}}
}

func (m *DirectModel) Name() string { return m.name }

func (m *DirectModel) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		body, err := json.Marshal(map[string]any{
			"model": m.name,
			"messages": []map[string]string{
				{"role": "user", "content": BuildPrompt(req)},
			},
			"temperature": judgeTemperature(),
			"max_tokens":  directMaxTokens(),
		})
		if err != nil {
			yield(nil, err)
			return
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
			m.base+"/v1/chat/completions", bytes.NewReader(body))
		if err != nil {
			yield(nil, err)
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := m.hc.Do(httpReq)
		if err != nil {
			yield(nil, fmt.Errorf("vLLM 호출 실패: %w", err))
			return
		}
		defer resp.Body.Close()

		raw, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			// 본문을 잘라 함께 남긴다 — 400 은 대개 max_tokens·창 길이 문제다.
			yield(nil, fmt.Errorf("vLLM %d: %s", resp.StatusCode, clipRunes(string(raw), 300)))
			return
		}

		var out struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			yield(nil, fmt.Errorf("vLLM 응답을 못 읽었다: %w", err))
			return
		}
		if len(out.Choices) == 0 {
			yield(nil, fmt.Errorf("vLLM 이 답을 하나도 안 줬다"))
			return
		}

		yield(&model.LLMResponse{
			Content: &genai.Content{
				Role:  "model",
				Parts: []*genai.Part{{Text: out.Choices[0].Message.Content}},
			},
		}, nil)
	}
}

// FromEnv 는 환경에 맞는 모델을 만든다.
//
// 모델을 만드는 결정은 여기 한 곳에서만 한다. code-insight-engine 에서
// 만드는 자리가 셋이라 서로 다른 결정을 하고 있었고, 하나만 바꿨더니
// 나머지가 여전히 큐로 가서 답이 계속 흔들렸다.
func FromEnv(name, amqpURL string) model.LLM {
	if direct := strings.TrimSpace(os.Getenv("LLM_DIRECT_URL")); direct != "" {
		log.Printf("[LLM] %s 에 직접 묻는다 (큐를 거치지 않는다)", direct)
		return NewDirectModel(name, direct)
	}
	return NewRabbitMQModel(name, amqpURL)
}

// judgeTemperature 는 판정에 쓸 온도다. 판정은 흔들리면 안 된다.
func judgeTemperature() float64 {
	if v := strings.TrimSpace(os.Getenv("LLM_JUDGE_TEMPERATURE")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 && f <= 2 {
			return f
		}
	}
	return 0
}

// directMaxTokens 는 한 번에 받을 최대 길이다.
// 너무 낮으면 답이 중간에 잘려 파싱이 조용히 실패한다.
func directMaxTokens() int {
	if v := strings.TrimSpace(os.Getenv("LLM_MAX_TOKENS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 4096
}

func clipRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
