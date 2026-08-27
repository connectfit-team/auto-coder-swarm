package orchestrator

import (
	"fmt"
	"log"
	"time"

	"github.com/connectfit-team/auto-coder-swarm/internal/observability"
)

func (t *taskContext) execute() (RunResult, error) {
	log.Printf("🏁 [ACS] Starting task execution: %s (Repo: %s)", t.taskID, t.req.TargetRepo)
	observability.ActiveWorkers.Inc()
	defer observability.ActiveWorkers.Dec()

	start := time.Now()
	if err := t.prepareAnalysis(); err != nil {
		log.Printf("❌ [ACS] Analysis phase failed for %s: %v", t.taskID, err)
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
		log.Printf("🔄 [ACS] Task %s: Execution Attempt %d/3", t.taskID, attempt)
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
			log.Printf("⚠️ [ACS] Verification failed for %s (Attempt %d). Retrying...", t.taskID, attempt)
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
			log.Printf("✅ [ACS] Task %s successfully completed and reviewed.", t.taskID)
			// [Step 52: MSA Chain Reaction] Trigger impact analysis for other repos
			chainTasks, chainErr := t.triggerChainReaction()
			if chainErr == nil && len(chainTasks) > 0 {
				res.ChainTasks = chainTasks
			}
			return res, nil
		}
	}

	log.Printf("❌ [ACS] Task %s failed after maximum attempts.", t.taskID)
	return RunResult{RepoName: t.targetRepo}, fmt.Errorf("최대 시도 초과")
}
