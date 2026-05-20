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
	Query string `json:"query"`
}

type AnalysisResponse struct {
	Response string `json:"response"`
}

type KnowledgeUpdateRequest struct {
	RepoName string `json:"repo_name"`
	Summary  string `json:"summary"`
	Keywords string `json:"keywords"`
}

type SemanticSearchRequest struct {
	Query string `json:"query"`
	Limit uint64 `json:"limit"`
}

type RepoMatch struct {
	RepoName string  `json:"repo_name"`
	Summary  string  `json:"summary"`
	Keywords string  `json:"keywords"`
	RepoType string  `json:"repo_type"`
	Score    float32 `json:"score"`
}

type SearchResult struct {
	FilePath string  `json:"file_path"`
	Content  string  `json:"content"`
	Score    float32 `json:"score"`
}

func (c *Client) QueryOracle(ctx context.Context, query string) (string, error) {
	reqBody := AnalysisRequest{Query: query}
	b, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/analyze", bytes.NewBuffer(b))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)

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

func (c *Client) UpdateKnowledge(ctx context.Context, repoName, summary, keywords string) error {
	reqBody := KnowledgeUpdateRequest{
		RepoName: repoName,
		Summary:  summary,
		Keywords: keywords,
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

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("oracle returned error status: %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) SemanticSearch(ctx context.Context, query string, limit uint64) ([]SearchResult, error) {
	reqBody := SemanticSearchRequest{Query: query, Limit: limit}
	b, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v1/search", bytes.NewBuffer(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var results []SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}
	return results, nil
}

func (c *Client) NavigateRepos(ctx context.Context, query string) ([]RepoMatch, error) {
	reqBody := map[string]string{"query": query}
	b, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v1/navigate", bytes.NewBuffer(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var results []RepoMatch
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}
	return results, nil
}
