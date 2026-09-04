package insightclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// 값 하나를 더하는 작업의 계획을 CIE 에서 받아 온다.
//
// 계획을 만드는 규칙은 CIE 에만 있다. 여기서 따로 만들면 두 규칙이 어긋나고,
// 어긋난 쪽이 조용히 틀린 PR 을 연다.

// VariantChange 는 파일 한 곳에 넣을 것이다.
type VariantChange struct {
	File        string   `json:"file"`         // 저장소 이름을 포함한 경로
	InsertAfter int      `json:"insert_after"` // 이 줄 다음에 넣는다(1부터)
	Block       []string `json:"block"`
	Anchor      string   `json:"anchor"`
	AnchorLine  int      `json:"anchor_line"` // 계획을 세울 때 그 줄의 번호(1부터)
}

// VariantRepoPlan 은 저장소 하나의 작업이다.
type VariantRepoPlan struct {
	Repo string `json:"repo"`
	// 워크트리를 딸 사본의 경로. 서브모듈이면 껍데기 안의 경로다 —
	// proto-userapis 는 따로 받아 두지 않고 protogen/userapis 로 있다.
	SourcePath string `json:"source_path"`
	Order      int    `json:"order"`
	Blocks     bool   `json:"blocks_others"`
	PushToMain bool   `json:"push_to_main"`
	// proto 는 PR 이 아니라 protogen 의 make 목표로 배포한다.
	Publish    string `json:"publish"`
	MakeTarget string `json:"make_target"`
	// 코드를 고치기 전에 갱신해야 할 의존. 없으면 소비자 PR 이 빌드에서 깨진다.
	DepBumps []DepBump `json:"dep_bumps"`
	// 이미 값이 들어 있어 넣을 것이 없던 자리. 절반만 적용된 상태를 알린다.
	AlreadyThere int             `json:"already_there"`
	Note         string          `json:"note"`
	Changes      []VariantChange `json:"changes"`
	NeedsManual  []string        `json:"needs_manual"`
}

// VariantPlanRequest 는 무엇을 더할지다.
type VariantPlanRequest struct {
	Seed    string   `json:"seed"`
	Value   string   `json:"value"`
	Label   string   `json:"label"`
	Peers   []string `json:"peers,omitempty"`
	Exclude []string `json:"exclude,omitempty"`
}

// VariantPlan 은 저장소별 작업 계획을 순서대로 받아 온다.
func (c *Client) VariantPlan(ctx context.Context, req VariantPlanRequest) ([]VariantRepoPlan, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/api/v1/variant/plan", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.hc.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("계획 요청 실패: HTTP %d", resp.StatusCode)
	}

	var out struct {
		Plans []VariantRepoPlan `json:"plans"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Plans, nil
}

// DepBump 는 코드를 고치기 전에 갱신해야 할 의존이다.
type DepBump struct {
	Kind   string `json:"kind"` // go | pubspec | vendored
	File   string `json:"file"`
	Module string `json:"module"`
	From   string `json:"from"`
}
