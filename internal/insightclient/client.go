package insightclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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
		return nil, fmt.Errorf("inventory request failed: %d", resp.StatusCode)
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
		return nil, fmt.Errorf("files request failed: %d", resp.StatusCode)
	}

	var result []string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

// CandidateFile 은 작업 요청과 관련된 파일이다. Hits 는 걸린 **낱말 수**다.
type CandidateFile struct {
	Path string `json:"path"`
	Hits int    `json:"hits"`
}

// FindCandidates 는 요청과 관련된 파일을 찾아 온다.
//
// 그동안은 저장소 파일 목록을 3000자로 잘라 모델에게 보여 주고 고르게 했다.
// cms 는 파일이 1130개라 사실상 제비뽑기였고, "월별 근무 조회 말일 누락" 에
// 이름이 비슷한 workstamp.ts 를 골랐다 — 정답은 workplace/workplace.ts 였다.
// 목록을 보여 주는 대신 찾아서 준다.
func (c *Client) FindCandidates(ctx context.Context, repo, request string, limit int) ([]CandidateFile, error) {
	u := fmt.Sprintf("%s/api/v1/candidates?repo=%s&q=%s&limit=%d",
		c.baseURL, url.QueryEscape(repo), url.QueryEscape(request), limit)
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
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
		return nil, fmt.Errorf("CIE candidates: %s", resp.Status)
	}

	var out struct {
		Files []CandidateFile `json:"files"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Files, nil
}
