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
	safeDiff := diff
	diffRunes := []rune(safeDiff)
	if len(diffRunes) > 20000 {
		safeDiff = string(diffRunes[:20000]) + "\n... (Diff truncated for safety) ..."
	}

	reqBody := ImpactAnalysisRequest{
		SourceRepo: repo,
		CodeDiff:   safeDiff,
	}
	b, _ := json.Marshal(reqBody)

	// **CIE 가 여는 경로는 이것이다.** 여태 /api/v1/impact/analyze 로 불렀고
	// 줄곧 404 였다 — 다중 저장소 연쇄가 한 번도 돈 적이 없다는 뜻이다.
	// 연결보류처럼 프런트에 새 RPC 가 필요한 일에서, 뒤쪽 저장소에 작업을
	// 만들어 주는 것이 이 길인데 그것이 막혀 있었으니 모델이 없는 RPC 를
	// 지어냈다(W-19079 의 updateInviteStatus).
	url := c.baseURL + "/api/v1/dependencies/analyze-impact"
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
