package ckhclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
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
		apiKey:  "ckh_team_secret_2026",
		hc: &http.Client{
			Timeout: 30 * time.Second, // Reduced for interactive responsiveness
		},
	}
}

type ContextRequest struct {
	WorkID          string `json:"work_id"`
	TaskDescription string `json:"task_description"`
	RepoContext     string `json:"repo_context"`
}

type ContextResponse struct {
	Summary          string   `json:"summary"`
	RelevantLinks    []string `json:"relevant_links"`
	SuggestedActions []string `json:"suggested_actions"`
}

func (c *Client) GetContextReport(ctx context.Context, workID, taskDesc, repo string) (*ContextResponse, error) {
	log.Printf("[CKH] Requesting policy report for %s", workID)
	reqBody := ContextRequest{
		WorkID:          workID,
		TaskDescription: taskDesc,
		RepoContext:     repo,
	}
	b, _ := json.Marshal(reqBody)

	url := c.baseURL + "/api/v1/context/report"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(b))
	if err != nil { return nil, err }
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.hc.Do(req)
	if err != nil { 
		log.Printf("⚠️ [CKH] Request failed: %v", err)
		return nil, err 
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("⚠️ [CKH] Error status %d", resp.StatusCode)
		return nil, fmt.Errorf("ckh error: %d", resp.StatusCode)
	}

	var result ContextResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil { return nil, err }
	log.Printf("[CKH] Knowledge retrieved for %s", workID)
	return &result, nil
}
