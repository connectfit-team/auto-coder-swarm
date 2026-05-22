package orchestrator

import (
	"fmt"
	"time"

	"github.com/connectfit-team/auto-coder-swarm/internal/observability"
)

func (t *taskContext) execute() (RunResult, error) {
	observability.ActiveWorkers.Inc()
	defer observability.ActiveWorkers.Dec()

	start := time.Now()
	if err := t.prepareAnalysis(); err != nil {
		return RunResult{}, err
	}
	observability.RecordStepDuration("analysis", t.targetRepo, time.Since(start).Seconds())

	wsPath, err := t.orchestrator.wsMgr.CreateWorkspace()
	if err != nil {
		return RunResult{}, err
	}
	t.wsPath = wsPath
	defer t.orchestrator.wsMgr.Cleanup(wsPath)

	for attempt := 1; attempt <= 3; attempt++ {
		if t.ctx.Err() != nil {
			return RunResult{}, t.ctx.Err()
		}

		startPlan := time.Now()
		if err := t.stepPlanning(attempt); err != nil {
			observability.IncrementAgentOp("Planner", "failed")
			return RunResult{}, err
		}
		observability.RecordStepDuration("planning", t.targetRepo, time.Since(startPlan).Seconds())
		observability.IncrementAgentOp("Planner", "success")

		startExec := time.Now()
		if err := t.stepExecution(attempt); err != nil {
			observability.IncrementAgentOp("Coder", "failed")
			return RunResult{}, err
		}
		observability.RecordStepDuration("execution", t.targetRepo, time.Since(startExec).Seconds())
		observability.IncrementAgentOp("Coder", "success")

		startVerif := time.Now()
		success, err := t.stepVerification()
		if err != nil {
			observability.RecordStepDuration("verification", t.targetRepo, time.Since(startVerif).Seconds())
			return RunResult{}, err
		}
		observability.RecordStepDuration("verification", t.targetRepo, time.Since(startVerif).Seconds())
		if !success {
			continue
		}

		startReview := time.Now()
		finished, res, err := t.stepReview()
		if err != nil {
			observability.RecordStepDuration("review", t.targetRepo, time.Since(startReview).Seconds())
			return RunResult{}, err
		}
		observability.RecordStepDuration("review", t.targetRepo, time.Since(startReview).Seconds())
		
		if finished {
			// [Step 52: MSA Chain Reaction] Trigger impact analysis for other repos
			chainTasks, chainErr := t.triggerChainReaction()
			if chainErr == nil && len(chainTasks) > 0 {
				res.ChainTasks = chainTasks
			}
			return res, nil
		}
	}

	return RunResult{RepoName: t.targetRepo}, fmt.Errorf("최대 시도 초과")
}
