package orchestrator

import (
	"context"
	"fmt"
	"log"
	"time"
)

func (o *SwarmOrchestrator) logDeepTechnical(ctx context.Context, taskID string, stage, message, prompt, rawResult string) {
	log.Printf("[%s] [%s] %s", taskID, stage, message)

	if o.store != nil {
		o.store.AddDeepLog(taskID, stage, message, prompt, "") // Summary disabled for speed
		task, _ := o.store.GetTaskByID(taskID)
		if task != nil {
			newState := fmt.Sprintf("%s\n[%s] %s: %s", task.ContextState, time.Now().Format("15:04:05"), stage, message)
			o.store.UpdateContextState(taskID, newState)
		}
	}
}
