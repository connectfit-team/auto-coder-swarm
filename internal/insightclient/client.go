package insightclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
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
		apiKey:  os.Getenv("CIE_API_KEY"),
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

	// **서버는 봉투에 담아 준다.** {"repo":…,"files":[…],"count":N}
	//
	// 여기서 []string 으로 받고 있었다. 디코드가 늘 실패했고 그 오류를
	// 호출부가 _ 로 버려서, 파일 목록은 **한 번도 온 적이 없다.** 그런데도
	// 아무 데서도 티가 안 났다 — 목록이 빈 채로 프롬프트에 들어가고,
	// 없는 경로를 걸러 내려던 검사도 조용히 건너뛰었다.
	var env struct {
		Files []string `json:"files"`
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(body, &env); err == nil && env.Files != nil {
		return env.Files, nil
	}
	// 옛 모양(배열 그대로)도 받는다.
	var plain []string
	if err := json.Unmarshal(body, &plain); err != nil {
		return nil, fmt.Errorf("files 응답을 읽지 못했다: %w", err)
	}
	return plain, nil
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
