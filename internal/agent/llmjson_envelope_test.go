package agent

import (
	"context"
	"encoding/json"
	"testing"
)

type strat struct {
	TotalFiles int      `json:"total_files"`
	IsFeasible bool     `json:"is_feasible"`
	Paths      []string `json:"actionable_path"`
}

// 실측으로 나온 응답이다. **오류 없이** 언마샬되고 전 필드가 zero 로 남아,
// is_feasible=false 가 되어 상위에서 "작업 규모 과다" 로 죽었다.
// 모델은 제대로 답했는데 판단해 보지도 못하고 실패한 것이다.
func TestUnmarshalUnwrapsResponseEnvelope(t *testing.T) {
	raw := `{"response":{"total_files":3,"is_feasible":true,"actionable_path":["a.ts","b.ts"]}}`
	var got strat
	if err := unmarshalMaybeWrapped([]byte(raw), &got); err != nil {
		t.Fatal(err)
	}
	if got.TotalFiles != 3 || !got.IsFeasible || len(got.Paths) != 2 {
		t.Errorf("봉투를 못 벗겼다: %+v", got)
	}
}

func TestUnmarshalKeepsPlainObject(t *testing.T) {
	var got strat
	if err := unmarshalMaybeWrapped([]byte(`{"total_files":5,"is_feasible":true}`), &got); err != nil {
		t.Fatal(err)
	}
	if got.TotalFiles != 5 || !got.IsFeasible {
		t.Errorf("봉투가 없는 것을 건드리면 안 된다: %+v", got)
	}
}

// 진짜로 비어 있는 답과 봉투를 구별해야 한다. 벗길 것이 없으면 그대로 둔다.
func TestUnmarshalLeavesTrulyEmpty(t *testing.T) {
	var got strat
	if err := unmarshalMaybeWrapped([]byte(`{"total_files":0,"is_feasible":false}`), &got); err != nil {
		t.Fatal(err)
	}
	if got.TotalFiles != 0 || got.IsFeasible {
		t.Errorf("없는 값을 지어내면 안 된다: %+v", got)
	}
}

// 키가 여럿이면 아는 봉투 이름일 때만 벗긴다 — 아무 키나 열면 엉뚱한 것을 읽는다.
func TestUnmarshalIgnoresUnknownWrapper(t *testing.T) {
	var got strat
	raw := `{"meta":{"total_files":9},"note":{"is_feasible":true}}`
	if err := unmarshalMaybeWrapped([]byte(raw), &got); err != nil {
		t.Fatal(err)
	}
	if got.TotalFiles != 0 {
		t.Errorf("모르는 키를 열면 안 된다: %+v", got)
	}
}

// 봉투를 벗기는 길이 재시도 루프 안에서도 도는지 본다.
func TestCallJSONWithUnwraps(t *testing.T) {
	var got strat
	_, err := callJSONWith(context.Background(), "p", &got, "T", func(string) (string, error) {
		return `{"result":{"total_files":2,"is_feasible":true}}`, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalFiles != 2 || !got.IsFeasible {
		t.Errorf("루프 안에서 안 벗겨졌다: %+v", got)
	}
}

var _ = json.Marshal
