package orchestrator

import (
	"testing"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
)

// 연쇄가 콕 집어 준 파일은 이번 계획에 없어도 되돌리면 안 된다.
//
// 실측 W-23965: 앞 회차에 `connect.service.proto` 에 RPC 를 바르게 더했는데,
// 되돌리기를 없앤 뒤(#182) 그 수정이 남아 있다가 이번 계획에 그 파일이 없어
// 「계획에 없는 파일」로 지워졌다. 필드만 있고 **바꾸는 길이 없는 반쪽**이
// 승인 대기까지 갔고, 관문도 비평가도 아무 말을 안 했다 — diff 에서 통째로
// 사라졌으니 볼 것이 없었다.
func TestPointedFileSurvivesAPlanThatForgotIt(t *testing.T) {
	tc := &taskContext{req: StatelessRequest{
		TargetFiles: []string{"ceoweb/v1/connect.communication.proto", "ceoweb/v1/connect.service.proto"},
	}}
	plan := agent.Plan{Changes: []agent.FileChange{{FilePath: "ceoweb/v1/connect.communication.proto"}}}

	if !tc.plannedOrPointedAt(plan, "ceoweb/v1/connect.service.proto") {
		t.Fatal("짚어 준 파일을 「계획에 없다」고 본다 — RPC 가 지워진다")
	}
	if !tc.plannedOrPointedAt(plan, "ceoweb/v1/connect.communication.proto") {
		t.Fatal("계획에 있는 파일을 「없다」고 본다")
	}
}

// 관문의 뜻은 **요청하지 않은 자리가 딸려 오는 것**을 막는 것이다.
// 그 뜻까지 풀어 버리면 안 된다.
func TestStrayFileIsStillReverted(t *testing.T) {
	tc := &taskContext{req: StatelessRequest{
		TargetFiles: []string{"ceoweb/v1/connect.service.proto"},
	}}
	plan := agent.Plan{Changes: []agent.FileChange{{FilePath: "ceoweb/v1/connect.service.proto"}}}
	if tc.plannedOrPointedAt(plan, "internal/export/api.go") {
		t.Fatal("딸려 온 파일을 통과시킨다 — 이 관문이 있는 까닭이 사라진다")
	}
}

// 짚어 준 것이 없으면 전과 똑같이 동작한다.
func TestNoTargetFilesBehavesAsBefore(t *testing.T) {
	tc := &taskContext{}
	plan := agent.Plan{Changes: []agent.FileChange{{FilePath: "a.proto"}}}
	if !tc.plannedOrPointedAt(plan, "a.proto") {
		t.Fatal("계획에 있는 파일을 막는다")
	}
	if tc.plannedOrPointedAt(plan, "b.proto") {
		t.Fatal("아무 파일이나 통과시킨다")
	}
}
