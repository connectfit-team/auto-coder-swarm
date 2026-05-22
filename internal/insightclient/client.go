package insightclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/connectfit-team/auto-coder-swarm/internal/bus"
)

type Client struct {
	baseURL string
	apiKey  string
	hc      *http.Client
	bus     *bus.MessageBus
}

func NewClient(baseURL string, mb *bus.MessageBus) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  "gig_team_secret_2026",
		hc: &http.Client{
			Timeout: 10 * time.Minute,
		},
		bus: mb,
	}
}

type AnalysisRequest struct {
	Query     string `json:"query"`
	SessionID string `json:"session_id,omitempty"`
}

type AnalysisResponse struct {
	WorkID string `json:"work_id"`
	Status string `json:"status"`
}

type ResultResponse struct {
	Response string `json:"response"`
}

func (c *Client) GetRepoInventory(ctx context.Context, repoName string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/api/v1/repos/inventory/%s", c.baseURL, repoName)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil { return nil, err }
	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.hc.Do(req)
	if err != nil { return nil, err }
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK { return nil, fmt.Errorf("inventory request failed: %d", resp.StatusCode) }

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil { return nil, err }
	return result, nil
}

func (c *Client) GetRepoFiles(ctx context.Context, repoName string, extension string, depth int) ([]string, error) {
	url := fmt.Sprintf("%s/api/v1/repos/inventory/%s/files?extension=%s&depth=%d", c.baseURL, repoName, extension, depth)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil { return nil, err }
	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.hc.Do(req)
	if err != nil { return nil, err }
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK { return nil, fmt.Errorf("files request failed: %d", resp.StatusCode) }

	var result []string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil { return nil, err }
	return result, nil
}
