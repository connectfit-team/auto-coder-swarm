package insightclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

func (c *Client) QueryOracle(ctx context.Context, query, sessionID string, onWorkID func(string)) (string, string, error) {
	log.Printf("[CIE] Submitting analysis request (Session: %s)", sessionID)
	
	reqBody := AnalysisRequest{Query: query, SessionID: sessionID}
	b, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/analyze", bytes.NewBuffer(b))
	if err != nil { return "", "", fmt.Errorf("failed to create request: %w", err) }
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.hc.Do(req)
	if err != nil {
		log.Printf("[CIE] Submit failed: %v", err)
		return "", "", fmt.Errorf("oracle submit failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		log.Printf("[CIE] Submit returned error status: %d", resp.StatusCode)
		return "", "", fmt.Errorf("oracle returned error status: %d", resp.StatusCode)
	}

	var ar AnalysisResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil { return "", "", fmt.Errorf("failed to decode oracle response: %w", err) }

	workID := ar.WorkID
	if workID == "" { return "", "", fmt.Errorf("oracle did not return a work_id") }

	log.Printf("[CIE] Task accepted. WorkID: %s", workID)
	if onWorkID != nil { onWorkID(workID) }

	// Poll for result with backoff
	delay := 3 * time.Second
	for i := 0; i < 60; i++ {
		select {
		case <-ctx.Done(): return "", workID, ctx.Err()
		case <-time.After(delay):
			log.Printf("[CIE] Polling result for %s (Attempt %d)", workID, i+1)
			resURL := fmt.Sprintf("%s/api/tasks/result?id=%s", c.baseURL, workID)
			rReq, _ := http.NewRequestWithContext(ctx, "GET", resURL, nil)
			rReq.Header.Set("X-API-Key", c.apiKey)

			rResp, err := c.hc.Do(rReq)
			if err != nil { continue }

			if rResp.StatusCode == http.StatusOK {
				var result ResultResponse
				if err := json.NewDecoder(rResp.Body).Decode(&result); err == nil && result.Response != "" {
					rResp.Body.Close()
					log.Printf("[CIE] Task %s completed", workID)
					return result.Response, workID, nil
				}
			}
			rResp.Body.Close()
			if delay < 15*time.Second { delay += 1 * time.Second }
		}
	}
	return "", workID, fmt.Errorf("oracle timeout for %s", workID)
}

func (c *Client) StopTask(ctx context.Context, workID string) error {
	if workID == "" { return nil }
	log.Printf("[CIE] Cancelling remote task: %s", workID)
	url := fmt.Sprintf("%s/api/v1/tasks/cancel?id=%s", c.baseURL, workID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil { return err }
	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.hc.Do(req)
	if err != nil { return err }
	defer resp.Body.Close()
	return nil
}

func (c *Client) UpdateKnowledge(ctx context.Context, repoName, summary, keywords string) error {
	reqBody := map[string]string{ "repo_name": repoName, "summary": summary, "keywords": keywords }
	b, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/update_knowledge", bytes.NewBuffer(b))
	if err != nil { return err }
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)
	resp, err := c.hc.Do(req)
	if err != nil { return err }
	defer resp.Body.Close()
	return nil
}
