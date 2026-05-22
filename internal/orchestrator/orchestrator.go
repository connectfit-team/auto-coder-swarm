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
	tc := o.newTaskContext(ctx, taskID, req, isApproved, repoLockFunc)
	return tc.execute()
}
