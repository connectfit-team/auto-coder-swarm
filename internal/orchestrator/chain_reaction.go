package orchestrator

import (
	"fmt"
	"sort"
	"strings"
)

// rootTaskID 는 이 연쇄의 뿌리 작업이다. 손자까지 한 줄에 묶는다.
func rootTaskID(t *taskContext) string {
	if t.req.ParentTaskID != "" {
		return t.req.ParentTaskID
	}
	return t.taskID
}

func (t *taskContext) triggerChainReaction() ([]StatelessRequest, error) {
	if t.req.Depth <= 0 {
		return nil, nil
	}

	if t.finalDiff == "" {
		return nil, nil
	}

	// **펴내기 전에는 쓰는 쪽이 달라지지 않는다.**
	//
	// 계약 원본(.proto)만 고친 것은 아직 아무에게도 보이지 않는다. 생성물은
	// 사람이 protogen 에서 make push-*apis 를 돌려야 만들어지고 밀린다.
	// 그런데 여기서 영향 분석을 돌려 쓰는 쪽 저장소마다 일을 만들었다 —
	// 실측으로 한 회차가 아홉 저장소에 열다섯 개를 만들었고, 근거는 지어낸
	// `Status` 라는 흔한 이름이 거기에도 있다는 것뿐이었다.
	if onlyContractSources(t.finalDiff) {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CHAIN_REACTION",
			"계약 원본만 고쳤다 — 펴내기 전에는 쓰는 쪽이 달라지지 않는다", "", "")
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
	var dropped []string

	// 확신이 높은 것부터 본다. 묶을 때 아무거나 남으면 안 된다.
	sort.SliceStable(impact.ImpactAnalysis, func(i, j int) bool {
		return impact.ImpactAnalysis[i].ConfidenceScore > impact.ImpactAnalysis[j].ConfidenceScore
	})

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
			// 형제 작업을 한 줄에 묶어 보여 주려면 뿌리를 알아야 한다.
			ParentTaskID: rootTaskID(t),
		}
		if len(triggeredTasks) >= maxImpactTasks {
			// **퍼지는 개수를 묶는다.** 이름 하나가 겹쳤다고 저장소 열
			// 곳에 일이 생기면, 사람이 보는 것은 일이 아니라 소음이다.
			dropped = append(dropped, impacted.RepoName)
			continue
		}
		triggeredTasks = append(triggeredTasks, newReq)

		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CHAIN_TRIGGERED",
			fmt.Sprintf("연쇄 작업 트리거: %s", impacted.RepoName),
			fmt.Sprintf("사유: %s (Confidence: %.2f)", impacted.Reason, impacted.ConfidenceScore), "")
	}

	if len(dropped) > 0 {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CHAIN_CAPPED",
			fmt.Sprintf("영향이 있다는 저장소가 많아 %d 곳만 남겼다", maxImpactTasks),
			strings.Join(dropped, ", "), "")
	}
	return triggeredTasks, nil
}

// 한 번에 일을 만들 저장소 수. 이름 하나가 겹쳤다고 열 곳에 일이 생기면
// 사람이 보는 것은 일이 아니라 소음이다.
const maxImpactTasks = 3

// onlyContractSources 는 고친 것이 계약 원본뿐인지 본다.
func onlyContractSources(diff string) bool {
	files := changedFiles(diff)
	if len(files) == 0 {
		return false
	}
	for _, f := range files {
		if !strings.HasSuffix(f, ".proto") {
			return false
		}
	}
	return true
}
