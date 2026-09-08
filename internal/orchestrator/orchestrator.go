package orchestrator

import (
	"context"

	"github.com/connectfit-team/auto-coder-swarm/internal/ckhclient"
	"github.com/connectfit-team/auto-coder-swarm/internal/gitmgr"
	"github.com/connectfit-team/auto-coder-swarm/internal/insightclient"
	"github.com/connectfit-team/auto-coder-swarm/internal/security"
	"github.com/connectfit-team/auto-coder-swarm/internal/storage"
	"github.com/connectfit-team/auto-coder-swarm/internal/workspace"
)

type SwarmOrchestrator struct {
	insightClient *insightclient.Client
	ckhClient     *ckhclient.Client
	wsMgr         workspace.Manager
	gitMgr        *gitmgr.GitManager
	store         *storage.Storage
	securityGuard *security.Guardrail
}

func NewSwarmOrchestrator(ic *insightclient.Client, cc *ckhclient.Client, ws workspace.Manager, gm *gitmgr.GitManager, s *storage.Storage, sg *security.Guardrail) *SwarmOrchestrator {
	return &SwarmOrchestrator{
		insightClient: ic,
		ckhClient:     cc,
		wsMgr:         ws,
		gitMgr:        gm,
		store:         s,
		securityGuard: sg,
	}
}

func (o *SwarmOrchestrator) RunStatelessTask(ctx context.Context, taskID string, req StatelessRequest, isApproved bool, repoLockFunc func(string) (bool, error)) (RunResult, error) {
	// 화면은 깊이를 안 보낸다. 그러면 연쇄가 아예 돌지 않아 결함 흐름이
	// 늘 한 저장소만 고친다 — 여러 저장소에 걸친 요청은 답이 될 수 없다.
	req.Depth = chainDepth(req.Depth)
	tc := o.newTaskContext(ctx, taskID, req, isApproved, repoLockFunc)
	return tc.execute()
}
