package insightclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// 요청문 하나를 넘기면 값 추가인지 판단하고 계획까지 돌려준다.
//
// 값을 뽑고 씨앗을 정하는 규칙은 CIE 에만 둔다. 여기서 따로 뽑으면 두 규칙이
// 어긋나고, 어긋난 쪽이 조용히 틀린 PR 을 연다.

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
	if resp.StatusCode != http.StatusOK {
		return out, fmt.Errorf("값 추가 판단 실패: HTTP %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return out, err
	}
	return out, nil
}
