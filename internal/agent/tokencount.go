package agent

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// 토큰은 **세는 것이지 어림하는 것이 아니다.**
//
// 어림으로만 묶었더니 코드에서 4% 모자랐다 — 어림 11,776 토큰짜리 프롬프트가
// 실제로는 12,263 이었고, 출력 몫 4,096 을 더하면 한계 16,384 에서 25토큰
// 남았다. 한 글자만 더 길었으면 400 이고, 그 시도는 통째로 날아간다.
//
// 서버에 /tokenize 가 있고 14ms 다. 붙지 않을 때만 어림으로 돌아간다.
var (
	tokenizerOnce sync.Once
	tokenizerURL  string
	tokenizerHC   = &http.Client{Timeout: 5 * time.Second}
	tokenizerDead bool
	tokenizerMu   sync.Mutex

	modelOnce   sync.Once
	servedModel string
)

func tokenizeEndpoint() string {
	tokenizerOnce.Do(func() {
		base := strings.TrimSpace(os.Getenv("LLM_DIRECT_URL"))
		if base == "" {
			base = "http://127.0.0.1:8000"
		}
		tokenizerURL = strings.TrimRight(base, "/") + "/tokenize"
	})
	return tokenizerURL
}

// modelName 은 /tokenize 에 실어 보낼 모델 이름이다.
//
// 코드에 박아 두면 서버가 다른 모델로 뜨는 날 말없이 어긋난다 — 400 이 나고
// 어림으로 물러서는데, 그 물러섬은 **보이지 않는다.** 서버에 물어서 쓴다.
func modelName() string {
	if n := strings.TrimSpace(os.Getenv("LLM_MODEL")); n != "" {
		return n
	}
	modelOnce.Do(func() {
		resp, err := tokenizerHC.Get(strings.TrimSuffix(tokenizeEndpoint(), "/tokenize") + "/v1/models")
		if err != nil {
			return
		}
		defer resp.Body.Close()
		var out struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if json.NewDecoder(resp.Body).Decode(&out) == nil && len(out.Data) > 0 {
			servedModel = out.Data[0].ID
		}
	})
	if servedModel != "" {
		return servedModel
	}
	return "gemma4:31b"
}

// CountTokens 는 실제로 센다. 못 세면 어림한다 — 어림은 많이 세는 쪽이다.
func CountTokens(s string) int {
	if s == "" {
		return 0
	}
	tokenizerMu.Lock()
	dead := tokenizerDead
	tokenizerMu.Unlock()
	if dead {
		return EstimateTokens(s)
	}

	body, err := json.Marshal(map[string]any{"model": modelName(), "prompt": s})
	if err != nil {
		return EstimateTokens(s)
	}
	resp, err := tokenizerHC.Post(tokenizeEndpoint(), "application/json", bytes.NewReader(body))
	if err != nil {
		// 한 번 실패하면 그 뒤로는 묻지 않는다 — 프롬프트마다 5초씩 기다릴 수 없다.
		tokenizerMu.Lock()
		tokenizerDead = true
		tokenizerMu.Unlock()
		return EstimateTokens(s)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// 400 은 대개 모델 이름이 안 맞는 것이다 — 고쳐지지 않으니 되풀이하지
		// 않는다. 한 번 세지 못했다고 프롬프트마다 헛걸음할 까닭이 없다.
		tokenizerMu.Lock()
		tokenizerDead = true
		tokenizerMu.Unlock()
		return EstimateTokens(s)
	}
	var out struct {
		Count int `json:"count"`
	}
	if json.NewDecoder(resp.Body).Decode(&out) != nil || out.Count <= 0 {
		return EstimateTokens(s)
	}
	return out.Count
}
