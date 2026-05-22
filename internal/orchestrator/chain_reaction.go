package orchestrator

import (
	"fmt"
)

func (t *taskContext) triggerChainReaction() ([]StatelessRequest, error) {
	if t.req.Depth <= 0 {
		return nil, nil
	}

	if t.finalDiff == "" {
		return nil, nil
	}

	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CHAIN_REACTION", "임팩트 분석 시작 (MSA Chain Reaction)", "", "")

	impact, err := t.orchestrator.insightClient.AnalyzeImpact(t.ctx, t.targetRepo, t.finalDiff)
	if err != nil {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "ERROR", "임팩트 분석 실패", err.Error(), "")
		return nil, err
	}

	if len(impact.ImpactAnalysis) == 0 {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CHAIN_REACTION", "영향을 받는 다른 레포지토리가 없습니다.", "", "")
		return nil, nil
	}

	var triggeredTasks []StatelessRequest
	
	// Create a new parent list for children
	newParents := append(t.req.ParentRepos, t.targetRepo)

	for _, impacted := range impact.ImpactAnalysis {
		// 1. Skip if it's the current repo (already handled by seenRepos logic implicitly, but let's be explicit)
		if impacted.RepoName == t.targetRepo {
			continue
		}

		// 2. Cycle Prevention: Check if the repo was already touched in this chain
		isCycle := false
		for _, p := range t.req.ParentRepos {
			if p == impacted.RepoName {
				isCycle = true
				break
			}
		}
		if isCycle {
			t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CYCLE_DETECTED", 
				fmt.Sprintf("순환 참조 감지 및 차단: %s", impacted.RepoName), "", "")
			continue
		}

		// 3. Confidence Threshold
		if impacted.ConfidenceScore < 0.7 {
			continue
		}

		newReq := StatelessRequest{
			UserRequest: fmt.Sprintf("[%s] 레포지토리의 변경으로 인해 영향이 예상됩니다: %s", t.targetRepo, impacted.Reason),
			TargetRepo:  impacted.RepoName,
			Depth:       t.req.Depth - 1,
			ParentRepos: newParents,
		}
		triggeredTasks = append(triggeredTasks, newReq)

		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CHAIN_TRIGGERED", 
			fmt.Sprintf("연쇄 작업 트리거: %s", impacted.RepoName), 
			fmt.Sprintf("사유: %s (Confidence: %.2f)", impacted.Reason, impacted.ConfidenceScore), "")
	}

	return triggeredTasks, nil
}
