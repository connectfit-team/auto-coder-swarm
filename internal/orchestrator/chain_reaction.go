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
	seenRepos := make(map[string]bool)
	seenRepos[t.targetRepo] = true

	for _, impacted := range impact.ImpactAnalysis {
		if seenRepos[impacted.RepoName] {
			continue
		}
		seenRepos[impacted.RepoName] = true

		if impacted.ConfidenceScore < 0.7 {
			continue
		}

		newReq := StatelessRequest{
			UserRequest: fmt.Sprintf("[%s] 레포지토리의 변경으로 인해 영향이 예상됩니다: %s", t.targetRepo, impacted.Reason),
			TargetRepo:  impacted.RepoName,
			Depth:       t.req.Depth - 1,
		}
		triggeredTasks = append(triggeredTasks, newReq)

		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CHAIN_TRIGGERED", 
			fmt.Sprintf("연쇄 작업 트리거: %s", impacted.RepoName), 
			fmt.Sprintf("사유: %s (Confidence: %.2f)", impacted.Reason, impacted.ConfidenceScore), "")
	}

	return triggeredTasks, nil
}
