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
	apiKey  string
	hc      *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  "gig_team_secret_2026",
		hc: &http.Client{
			Timeout: 10 * time.Minute,
		},
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

func (c *Client) QueryOracle(ctx context.Context, query, sessionID string, onWorkID func(string)) (string, string, error) {
	// 1. Submit asynchronous request
	reqBody := AnalysisRequest{Query: query, SessionID: sessionID}
	b, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/analyze", bytes.NewBuffer(b))
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.hc.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("oracle submit failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("oracle returned error status: %d", resp.StatusCode)
	}

	var ar AnalysisResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return "", "", fmt.Errorf("failed to decode oracle submit response: %w", err)
	}

	workID := ar.WorkID
	if workID == "" {
		return "", "", fmt.Errorf("oracle did not return a work_id")
	}

	// [Deep Tracking] Notify the caller about the workID immediately
	if onWorkID != nil {
		onWorkID(workID)
	}

	// 2. Poll for result (Step 44: Task Polling logic)
	for i := 0; i < 60; i++ { // Timeout after 5-10 mins roughly depending on CIE speed
		select {
		case <-ctx.Done():
			return "", workID, ctx.Err()
		case <-time.After(5 * time.Second):
			resURL := fmt.Sprintf("%s/api/tasks/result?id=%s", c.baseURL, workID)
			rReq, _ := http.NewRequestWithContext(ctx, "GET", resURL, nil)
			rReq.Header.Set("X-API-Key", c.apiKey)

			rResp, err := c.hc.Do(rReq)
			if err != nil {
				continue
			}
			
			if rResp.StatusCode == http.StatusOK {
				var result ResultResponse
				if err := json.NewDecoder(rResp.Body).Decode(&result); err == nil && result.Response != "" {
					rResp.Body.Close()
					return result.Response, workID, nil
				}
			}
			rResp.Body.Close()
		}
	}

	return "", workID, fmt.Errorf("oracle analysis timeout for work_id: %s", workID)
}

func (c *Client) StopTask(ctx context.Context, workID string) error {
	if workID == "" {
		return nil
	}
	// [API Update] CIE Stop API moved to /api/v1/tasks/cancel
	url := fmt.Sprintf("%s/api/v1/tasks/cancel?id=%s", c.baseURL, workID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("oracle cancel request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("oracle cancel returned error status: %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) GetRepoInventory(ctx context.Context, repoName string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/api/v1/repos/inventory/%s", c.baseURL, repoName)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("inventory request failed with status: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) GetRepoFiles(ctx context.Context, repoName string, extension string, depth int) ([]string, error) {
	url := fmt.Sprintf("%s/api/v1/repos/inventory/%s/files?extension=%s&depth=%d", c.baseURL, repoName, extension, depth)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("files request failed with status: %d", resp.StatusCode)
	}

	var result []string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) UpdateKnowledge(ctx context.Context, repoName, summary, keywords string) error {
	reqBody := map[string]string{
		"repo_name": repoName,
		"summary":  summary,
		"keywords": keywords,
	}
	b, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/update_knowledge", bytes.NewBuffer(b))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("oracle request failed: %w", err)
	}
	defer resp.Body.Close()

	return nil
}
