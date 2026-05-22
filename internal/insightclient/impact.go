package insightclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type ImpactAnalysisRequest struct {
	SourceRepo string `json:"source_repo"`
	CodeDiff   string `json:"code_diff"`
}

type ImpactedFile struct {
	RepoName        string  `json:"repo_name"`
	FilePath        string  `json:"file_path"`
	Reason          string  `json:"reason"`
	ConfidenceScore float64 `json:"confidence_score"`
}

type ImpactAnalysisResponse struct {
	SourceRepo     string         `json:"source_repo"`
	ImpactAnalysis []ImpactedFile `json:"impact_analysis"`
}

func (c *Client) AnalyzeImpact(ctx context.Context, repo, diff string) (*ImpactAnalysisResponse, error) {
	reqBody := ImpactAnalysisRequest{
		SourceRepo: repo,
		CodeDiff:   diff,
	}
	b, _ := json.Marshal(reqBody)

	url := c.baseURL + "/api/v1/impact/analyze"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(b))
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

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("impact analysis failed with status: %d", resp.StatusCode)
	}

	var result ImpactAnalysisResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}
