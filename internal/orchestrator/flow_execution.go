package orchestrator

import (
	"fmt"
	"path/filepath"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
)

func (t *taskContext) stepExecution(attempt int) error {
	plan := t.ctx.Value("current_plan").(agent.Plan)
	for _, change := range plan.Changes {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CODING", fmt.Sprintf("[%s] 수정", change.FilePath), change.Instructions, "")
		t.coder.ModifyFile(t.ctx, filepath.Join(t.repoPath, change.FilePath), change.Instructions)
	}
	return nil
}
