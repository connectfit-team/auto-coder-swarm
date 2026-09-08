package ckhclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

// 사내지식은 한 번도 붙은 적이 없었다.
//
// 이 클라이언트는 `POST /api/v1/context/report` 를 불렀다. CKH 에 그런 길은
// 없다 — 실제 길은 `POST /api/v1/ask` 로 물어 두고 `GET /api/v1/tasks/{id}`
// 로 받아 가는 비동기다. 그래서 모든 작업이 **404 를 받고 지식 없이** 돌았고,
// 그 실패는 "지식이 없으면 품질만 떨어진다" 는 판단으로 조용히 넘겨졌다.
//
// 조용히 넘긴 것이 문제였다. 실측으로 "고용주웹 연결보류" 를 CKH 에 물으면
// 이렇게 답한다.
//
//	"근로자에서 고용주 연결하는 건 일단 보류하기로 했는데" (2020-12-14)
//
// 「연결보류」는 기능 이름이 아니라 **연결 기능을 보류한 결정**이다. 그것을
// 알았다면 코드를 쓰기 전에 사람에게 물었을 것이다.

const (
	askPollEvery = 5 * time.Second
	askWaitMax   = 3 * time.Minute
)

type askRequest struct {
	Query  string `json:"query"`
	Effort string `json:"effort,omitempty"`
}

type askEnvelope struct {
	Success bool `json:"success"`
	Data    struct {
		TaskID int    `json:"task_id"`
		Status string `json:"status"`
		Poll   string `json:"poll"`
	} `json:"data"`
	Error string `json:"error"`
}

type taskEnvelope struct {
	Success bool `json:"success"`
	Data    struct {
		ID     int    `json:"id"`
		Status string `json:"status"`
		Report string `json:"report"`
		Error  string `json:"error"`
	} `json:"data"`
}

// Ask 는 사내지식에 묻고 답을 기다린다.
//
// 비동기라 기다려야 한다. 정해진 시간 안에 안 오면 빈 답을 준다 — 지식이
// 늦는다고 작업을 세우지는 않는다. 다만 **못 받았다는 사실은 부르는 쪽이
// 알아야 한다**(error 로 돌려준다).
func (c *Client) Ask(ctx context.Context, query, effort string) (string, error) {
	if query == "" {
		return "", fmt.Errorf("물을 것이 비었다")
	}
	b, _ := json.Marshal(askRequest{Query: query, Effort: effort})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/ask", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return "", fmt.Errorf("사내지식에 못 물었다: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return "", fmt.Errorf("사내지식이 %d 를 준다 (길이 바뀌었을 수 있다: /api/v1/ask)", resp.StatusCode)
	}
	var env askEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return "", err
	}
	if env.Data.TaskID == 0 {
		return "", fmt.Errorf("사내지식이 작업 번호를 안 준다: %s", env.Error)
	}

	deadline := time.Now().Add(askWaitMax)
	for {
		if time.Now().After(deadline) {
			return "", fmt.Errorf("사내지식이 %s 안에 답하지 않았다 (작업 %d)", askWaitMax, env.Data.TaskID)
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(askPollEvery):
		}
		rep, status, err := c.fetchAsk(ctx, env.Data.TaskID)
		if err != nil {
			return "", err
		}
		switch status {
		case "COMPLETED", "DONE":
			return rep, nil
		case "FAILED", "ERROR":
			return "", fmt.Errorf("사내지식이 실패했다 (작업 %d)", env.Data.TaskID)
		}
	}
}

func (c *Client) fetchAsk(ctx context.Context, id int) (report, status string, err error) {
	url := c.baseURL + "/api/v1/tasks/" + strconv.Itoa(id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", "", err
	}
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("사내지식 작업 조회가 %d 를 준다", resp.StatusCode)
	}
	var env taskEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return "", "", err
	}
	return env.Data.Report, env.Data.Status, nil
}

// CheckContract 는 시작할 때 길이 살아 있는지 한 번 본다.
//
// 조용히 404 를 받는 것이 가장 나쁘다 — 지식 없이 도는 것을 아무도 모른다.
func (c *Client) CheckContract(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("사내지식에 닿지 않는다: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("사내지식 health 가 %d 다", resp.StatusCode)
	}
	// ask 길이 살아 있는지도 본다. 빈 질문에는 400 이 와야 한다 —
	// 404 가 오면 길이 없는 것이다.
	req2, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/ask",
		bytes.NewReader([]byte(`{}`)))
	req2.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req2.Header.Set("X-API-Key", c.apiKey)
	}
	r2, err := c.hc.Do(req2)
	if err != nil {
		return fmt.Errorf("사내지식 /api/v1/ask 에 못 닿는다: %w", err)
	}
	r2.Body.Close()
	if r2.StatusCode == http.StatusNotFound {
		return fmt.Errorf("사내지식에 /api/v1/ask 길이 없다 — 계약이 바뀌었다")
	}
	log.Printf("[CKH] 사내지식 계약 확인 (ask=%d)", r2.StatusCode)
	return nil
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
