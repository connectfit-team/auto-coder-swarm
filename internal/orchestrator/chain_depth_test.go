package orchestrator

import (
	"encoding/json"
	"os"
	"testing"
)

// S-06 — 요청이 깊이를 안 적어도 연쇄가 돌아야 한다.
//
// 깊이가 0 이면 triggerChainReaction 이 첫 줄에서 돌아 나가고, 결함 흐름은
// 늘 한 저장소만 고친다. 화면의 PR 단추가 하나뿐이던 까닭이다.
func TestChainDepth(t *testing.T) {
	var want string
	for _, raw := range scopeProblemsRaw(t) {
		var p struct {
			ID     string `json:"id"`
			Expect string `json:"expect"`
		}
		if json.Unmarshal(raw, &p) == nil && p.ID == "S-06" {
			want = p.Expect
		}
	}
	if want == "" {
		t.Skip("목록에 S-06 가 없다")
	}

	os.Unsetenv("SWARM_CHAIN_DEPTH")
	if got := chainDepth(0); got < 1 {
		t.Errorf("깊이를 안 적었는데 %d 다 — 연쇄가 안 돈다. 기대: %s", got, want)
	}
	// 요청이 적었으면 그것을 따른다.
	if got := chainDepth(3); got != 3 {
		t.Errorf("요청이 3 을 적었는데 %d 다", got)
	}
	// 끄고 싶으면 끌 수 있어야 한다 — 옛 동작으로 돌아가는 길.
	t.Setenv("SWARM_CHAIN_DEPTH", "0")
	if got := chainDepth(0); got != 0 {
		t.Errorf("0 으로 껐는데 %d 다", got)
	}
	t.Setenv("SWARM_CHAIN_DEPTH", "2")
	if got := chainDepth(0); got != 2 {
		t.Errorf("2 로 정했는데 %d 다", got)
	}
	// 이상한 값은 무시하고 기본으로 간다.
	t.Setenv("SWARM_CHAIN_DEPTH", "이상한값")
	if got := chainDepth(0); got != defaultChainDepth {
		t.Errorf("이상한 값에 %d 다", got)
	}
}
