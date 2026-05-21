package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
)

func (o *SwarmOrchestrator) logDeepTechnical(ctx context.Context, taskID string, stage, message, prompt, rawResult string) {
	summary := ""
	if rawResult != "" {
		primary, _ := o.loadModels()
		summarizePrompt := fmt.Sprintf("기술 요약(Summary) 작성: %s", rawResult)
		summary, _ = agent.CallLLM(ctx, primary, "SummaryAgent", summarizePrompt)
	}
	if o.store != nil {
		o.store.AddDeepLog(taskID, stage, message, prompt, summary)
		task, _ := o.store.GetTaskByID(taskID)
		if task != nil {
			newState := fmt.Sprintf("%s\n[%s] %s: %s", task.ContextState, time.Now().Format("15:04:05"), stage, message)
			o.store.UpdateContextState(taskID, newState)
		}
	}
}
