package insightclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// RepoRoute 는 CIE 가 고른 저장소 하나와 그 이유다.
type RepoRoute struct {
	RepoName string  `json:"repo_name"`
	Why      string  `json:"why"`
	Score    float32 `json:"score"`
	Pinned   bool    `json:"pinned"`
}

// RouteRepos 는 이 요청이 어느 저장소를 가리키는지 CIE 에게 묻는다.
//
// **모델에게 이름을 묻지 않는다.** 그동안은 요청문을 LLM 에 넣어 저장소
// 이름을 뽑았다. 느린 데다 색인에 없는 이름을 지어낼 수 있었고, 아무것도
// 못 뽑으면 작업이 통째로 죽었다. 여기서는 임베딩 검색이라 빠르고,
// 실제로 받아 둔 저장소 중에서만 나온다.
func (c *Client) RouteRepos(ctx context.Context, request string) ([]RepoRoute, error) {
	u := fmt.Sprintf("%s/api/v1/repos/route?q=%s", c.baseURL, url.QueryEscape(request))
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
		return nil, fmt.Errorf("저장소 라우팅 실패: HTTP %d", resp.StatusCode)
	}

	var out struct {
		Repos []RepoRoute `json:"repos"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Repos, nil
}
