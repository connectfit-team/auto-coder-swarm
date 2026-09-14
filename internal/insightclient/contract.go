package insightclient

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
)

// 시작할 때 눈에게 거는 말이 통하는지 한 번 본다.
//
// **조용히 404 를 받는 것이 가장 나쁘다.** 다중 저장소 연쇄(임팩트 분석)가
// 줄곧 404 였는데 아무도 몰랐다 — ACS 는 /api/v1/impact/analyze 로 불렀고
// CIE 는 /api/v1/dependencies/analyze-impact 를 연다. 그 길이 막혀 있으면
// 프런트에 새 RPC 가 필요한 일에서 뒤쪽 저장소에 작업이 만들어지지 않고,
// 모델이 없는 RPC 를 지어낸다(W-19079 의 updateInviteStatus).
//
// 같은 검사가 사내지식 쪽에도 있었는데 **아무도 부르지 않았다.** 그래서
// 여기서는 부르는 자리까지 함께 둔다(cmd/swarm).

type probe struct {
	name   string
	method string
	path   string
	body   string
}

// 404 만 실수로 본다. 400·405·500 은 길이 있다는 뜻이다 — 우리가 보낸
// 맛보기 요청이 알맹이가 없어서 그런 것이고, 그건 여기서 볼 일이 아니다.
func (c *Client) CheckContract(ctx context.Context) error {
	probes := []probe{
		{"저장소 고르기", "GET", "/api/v1/repos/route?q=ping", ""},
		{"인벤토리", "GET", "/api/v1/repos/inventory/auto-coder-swarm", ""},
		{"후보 파일", "GET", "/api/v1/candidates?repo=auto-coder-swarm&q=ping&limit=1", ""},
		{"작업 절차", "GET", "/api/v1/skills?repo=auto-coder-swarm", ""},
		{"임팩트 분석", "POST", "/api/v1/dependencies/analyze-impact", `{"source_repo":"","code_diff":""}`},
		{"값 추가 묻기", "POST", "/api/v1/variant/ask", `{}`},
		{"값 추가 계획", "POST", "/api/v1/variant/plan", `{}`},
	}

	var dead []string
	for _, p := range probes {
		var body *bytes.Reader
		if p.body != "" {
			body = bytes.NewReader([]byte(p.body))
		} else {
			body = bytes.NewReader(nil)
		}
		req, err := http.NewRequestWithContext(ctx, p.method, c.baseURL+p.path, body)
		if err != nil {
			dead = append(dead, p.name+"(요청 못 만듦)")
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		if c.apiKey != "" {
			req.Header.Set("X-API-Key", c.apiKey)
		}
		resp, err := c.hc.Do(req)
		if err != nil {
			dead = append(dead, fmt.Sprintf("%s(못 닿음: %v)", p.name, err))
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			dead = append(dead, fmt.Sprintf("%s %s → 404", p.name, p.path))
		}
	}

	if len(dead) > 0 {
		return fmt.Errorf("눈과 말이 안 통하는 길이 있다: %s", strings.Join(dead, " · "))
	}
	log.Printf("[CIE] 계약 확인 — 길 %d개 모두 살아 있다", len(probes))
	return nil
}
