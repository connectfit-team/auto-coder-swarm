package gitmgr

import (
	"fmt"
	"os"
	"strings"
)

// 시험으로 만든 수정이 실제 저장소로 나가지 않게 한다.
//
// 이 시스템을 고도화하는 동안 같은 요청을 몇 번이고 돌린다. 그때 나오는
// 수정은 **무엇이 틀렸는지 보려고** 만든 것이지 쓰려고 만든 것이 아니다.
// 실제로 나온 것들이 이랬다 — 없는 RPC 를 부르는 코드, 요청과 상관없는 한
// 줄, 멀쩡한 주석에서 한 글자가 빠진 것.
//
// 그런데 승인 한 번이면 가지를 밀고 PR 까지 연다. 실수 한 번과 사고 한 번
// 사이에 아무것도 없었다.
//
// **기본값은 아무 데도 밀지 않는 것이다.** 밀어도 되는 저장소를
// SWARM_PUSH_ALLOW 에 하나씩 적어야 열린다. 비워 두면 막힌다 — 잊어서
// 위험해지는 쪽이 아니라, 잊으면 안전해지는 쪽으로 둔다.
//
//	SWARM_PUSH_ALLOW=            아무 데도 못 민다 (기본)
//	SWARM_PUSH_ALLOW=*           다 민다 (운영에서 켤 때만)
//	SWARM_PUSH_ALLOW=cms,worker  적힌 곳만

// pushAllowed 는 그 저장소로 밀어도 되는지 본다.
func pushAllowed(repoName string) (bool, string) {
	raw := strings.TrimSpace(os.Getenv("SWARM_PUSH_ALLOW"))
	if raw == "" {
		return false, "SWARM_PUSH_ALLOW 가 비어 있다 — 어느 저장소로도 밀지 않는다"
	}
	for _, part := range strings.Split(raw, ",") {
		p := strings.TrimSpace(part)
		if p == "*" {
			return true, ""
		}
		if strings.EqualFold(p, repoName) {
			return true, ""
		}
	}
	return false, fmt.Sprintf("%s 는 SWARM_PUSH_ALLOW 에 없다 (허용: %s)", repoName, raw)
}
