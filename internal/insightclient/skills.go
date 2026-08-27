package insightclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// SkillDoc 는 CIE 가 주는 팀의 작업 절차 문서 하나다.
type SkillDoc struct {
	Path    string `json:"path"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Always  bool   `json:"always"`
}

// GetSkills 는 이 작업에서 지켜야 할 절차 문서를 CIE 에서 받아 온다.
//
// **디스크를 직접 읽지 않는다.** 스킬 문서는 CIE 쪽에 한 벌만 있고, ACS 가 사본을
// 두면 어느 쪽이 최신인지 알 수 없게 된다. 탐색은 Eyes 의 몫이라는 경계와도 맞는다.
func (c *Client) GetSkills(ctx context.Context, text, repo string, exts []string, limit int) ([]SkillDoc, error) {
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("빈 질의로는 절차를 고를 수 없다")
	}

	q := url.Values{}
	q.Set("q", text)
	if repo != "" {
		q.Set("repo", repo)
	}
	for _, e := range exts {
		if e != "" {
			q.Add("ext", e)
		}
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}

	// 절차 조회는 분석과 달리 즉답이다. 여기서 오래 기다리면 작업 전체가 멎는다.
	rctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(rctx, "GET", c.baseURL+"/api/v1/skills?"+q.Encode(), nil)
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
		return nil, fmt.Errorf("skills request failed: %d", resp.StatusCode)
	}

	var out struct {
		Skills []SkillDoc `json:"skills"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Skills, nil
}
