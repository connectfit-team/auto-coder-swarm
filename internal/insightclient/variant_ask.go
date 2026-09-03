package insightclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// 요청문 하나를 넘기면 값 추가인지 판단하고 계획까지 돌려준다.
//
// 값을 뽑고 씨앗을 정하는 규칙은 CIE 에만 둔다. 여기서 따로 뽑으면 두 규칙이
// 어긋나고, 어긋난 쪽이 조용히 틀린 PR 을 연다.

// ErrNotAuthorized 는 CIE 가 열쇠를 거절했다는 뜻이다. 설정 문제지 판단 결과가
// 아니므로, 이것을 "값 추가가 아니다" 로 읽으면 안 된다.
var ErrNotAuthorized = errors.New("CIE 가 열쇠를 거절했다 (CIE_API_KEY)")

// VariantAskResult 는 요청문 하나에 대한 답이다.
type VariantAskResult struct {
	IsVariantAddition bool              `json:"is_variant_addition"`
	Words             []string          `json:"words"`
	Seed              string            `json:"seed"`
	Value             string            `json:"value"`
	Label             string            `json:"label"`
	Plans             []VariantRepoPlan `json:"plans"`
	Repos             int               `json:"repos"`
	Error             string            `json:"error"`
	Why               string            `json:"why"`
	// 씨앗이 나오는 저장소를 모두 세고 뺀 이유를 붙인 것.
	Coverage []RepoVerdict `json:"coverage"`
}

// VariantAsk 는 요청문을 그대로 넘겨 계획을 받아 온다.
func (c *Client) VariantAsk(ctx context.Context, request string, exclude []string) (VariantAskResult, error) {
	var out VariantAskResult

	body, err := json.Marshal(map[string]any{
		"request": request,
		"exclude": exclude,
	})
	if err != nil {
		return out, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/api/v1/variant/ask", bytes.NewReader(body))
	if err != nil {
		return out, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.hc.Do(req)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return out, ErrNotAuthorized
	}
	if resp.StatusCode != http.StatusOK {
		return out, fmt.Errorf("값 추가 판단 실패: HTTP %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return out, err
	}
	return out, nil
}

// RepoVerdict 는 저장소 하나를 왜 넣었는지, 왜 뺐는지다.
type RepoVerdict struct {
	Repo     string `json:"repo"`
	Hits     int    `json:"hits"`
	GenHits  int    `json:"gen_hits"`
	Planned  int    `json:"planned"`
	Reason   string `json:"reason"`
	Evidence string `json:"evidence"`
}
