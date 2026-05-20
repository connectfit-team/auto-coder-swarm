package insightclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	baseURL string
	hc      *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		hc: &http.Client{
			Timeout: 10 * time.Minute,
		},
	}
}

type AnalysisRequest struct {
	Query string `json:"query"`
}

type AnalysisResponse struct {
	Response string `json:"response"`
}

func (c *Client) QueryOracle(ctx context.Context, query string) (string, error) {
	reqBody := AnalysisRequest{Query: query}
	b, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/analyze", bytes.NewBuffer(b))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.hc.Do(req)
	if err != nil {
		return "", fmt.Errorf("oracle request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("oracle returned error status: %d", resp.StatusCode)
	}

	var res AnalysisResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", fmt.Errorf("failed to decode oracle response: %w", err)
	}

	return res.Response, nil
}
