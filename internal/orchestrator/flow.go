package orchestrator

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/connectfit-team/auto-coder-swarm/internal/observability"
)

func (t *taskContext) execute() (RunResult, error) {
	log.Printf("🏁 [ACS] Starting task execution: %s (Repo: %s)", t.taskID, t.req.TargetRepo)
	observability.ActiveWorkers.Inc()
	defer observability.ActiveWorkers.Dec()

	// **승인은 "본 것을 그대로 올린다" 는 뜻이다.**
	//
	// 그동안 승인하면 분석부터 전부 다시 돌았다. 5분 넘게 걸리는 것도 문제지만,
	// 더 나쁜 것은 **사람이 본 diff 와 올라가는 diff 가 달라질 수 있다**는 점이다.
	// 검토의 뜻이 사라진다. 이미 만들어 둔 변경이 있으면 그것만 올린다.
	if t.isApproved {
		if res, done, err := t.shipReviewedDiff(); done {
			return res, err
		}
	}

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
			// 되먹임을 주고 다시 세우면 되는 실패는 남은 시도를 쓴다.
			if errors.Is(err, errRetryPlanning) && attempt < 3 {
				continue
			}
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

	// **검토자가 반대해도 고친 것을 버리지는 않는다.**
	//
	// 검토자가 맞는 수정을 세 번 거부하고 작업이 통째로 실패한 일이 있었다.
	// 실측 — 말일 경계 버그를 정확히 고쳤는데(빌드 통과, 비평도 통과) 검토자가
	// "0 이 이번 달 말일이니 원래대로 두라" 며 되돌렸다. 맞는 말 같지만 lt 가
	// 그 말일 00:00 을 경계로 삼아 말일이 통째로 빠지는 것이 원래 버그였다.
	//
	// 사람에게 넘기면 5초면 판정할 일을, 아무것도 안 남기고 실패로 끝내면
	// 고친 것과 그 논쟁이 함께 사라진다. 고친 것이 남아 있으면 검토자의
	// 반대를 붙여 승인 대기로 보낸다.
	// 반려는 작업공간을 되돌린다. 그래서 마지막 시도가 반려되면 여기 오기
	// 전에 고친 것이 사라져, 바로 아래의 "버리지 않는다" 가 무력해진다 —
	// 실측으로 그렇게 끝난 실패가 4건이었다. 되돌리기 직전에 남겨 둔 것을 쓴다.
	diff := t.currentDiff()
	if diff == "" {
		diff = t.lastRejectedDiff
	}
	if diff != "" {
		log.Printf("⚖️ [ACS] Task %s: 검토자와 뜻이 다릅니다. 사람 판단으로 넘깁니다.", t.taskID)
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "HANDOVER_TO_HUMAN",
			"검토자가 반대했지만 고친 내용이 남아 있어 사람 판단으로 넘깁니다",
			"", clip(t.lastFeedback, 1500))
		t.orchestrator.store.UpdateTaskProposedDiff(t.taskID, diff)
		return RunResult{RepoName: t.targetRepo, WaitingApproval: true}, nil
	}

	log.Printf("❌ [ACS] Task %s failed after maximum attempts.", t.taskID)
	return RunResult{RepoName: t.targetRepo}, fmt.Errorf("최대 시도 초과")
}

// 되먹임을 주고 다시 세우면 되는 계획 실패. 남은 시도를 쓴다.
var errRetryPlanning = errors.New("계획을 다시 세운다")

// currentDiff 는 작업공간에 지금 남아 있는 변경을 준다.
//
// 검토에서 되돌려졌으면 비어 있다. 남아 있으면 사람이 볼 값어치가 있다.
func (t *taskContext) currentDiff() string {
	if t.repoPath == "" {
		return ""
	}
	out, err := exec.CommandContext(t.ctx, "git", "-C", t.repoPath, "diff", "HEAD").Output()
	if err != nil {
		return ""
	}
	d := string(out)
	if strings.TrimSpace(d) == "" || isCosmeticDiff(d) {
		return ""
	}
	return d
}

// shipReviewedDiff 는 사람이 본 변경을 그대로 올린다.
//
// 저장해 둔 diff 를 새 작업공간에 그대로 적용하고 push·PR 까지 간다.
// 다시 만들지 않는다 — 다시 만들면 다른 것이 나올 수 있고, 그러면 승인한
// 사람이 못 본 코드가 나간다.
//
// 저장된 diff 가 없거나 적용에 실패하면 done=false 로 돌려 평소 흐름을 탄다.
func (t *taskContext) shipReviewedDiff() (RunResult, bool, error) {
	task, err := t.orchestrator.store.GetTaskByID(t.taskID)
	if err != nil || task == nil || strings.TrimSpace(task.ProposedDiff) == "" {
		return RunResult{}, false, nil
	}
	repo := task.RepoName
	if repo == "" {
		repo = t.req.TargetRepo
	}
	if repo == "" {
		return RunResult{}, false, nil
	}

	wsPath, err := t.orchestrator.wsMgr.CreateWorkspace()
	if err != nil {
		return RunResult{}, false, nil
	}
	defer t.orchestrator.wsMgr.Cleanup(wsPath)

	t.targetRepo = repo
	t.wsPath = wsPath
	t.repoPath = filepath.Join(wsPath, "repo")
	t.currentBranch = fmt.Sprintf("acs-fix-%s", time.Now().Format("0102150405"))
	if err := t.orchestrator.wsMgr.CreateWorktree(repo, t.repoPath, t.currentBranch); err != nil {
		return RunResult{}, false, nil
	}

	patch := filepath.Join(wsPath, "approved.patch")
	if err := os.WriteFile(patch, []byte(task.ProposedDiff), 0o644); err != nil {
		return RunResult{}, false, nil
	}
	apply := exec.CommandContext(t.ctx, "git", "-C", t.repoPath, "apply", "--whitespace=nowarn", patch)
	if out, err := apply.CombinedOutput(); err != nil {
		// 저장소가 그 사이 바뀌었을 수 있다. 그때는 처음부터 다시 하는 편이 맞다.
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "REAPPLY_FAILED",
			"승인된 변경을 그대로 적용하지 못해 처음부터 다시 합니다", "", strings.TrimSpace(string(out)))
		return RunResult{}, false, nil
	}

	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "SHIP_REVIEWED",
		"사람이 본 변경을 그대로 올립니다 (다시 만들지 않음)", "", "")

	prURL, prErr := t.orchestrator.gitMgr.PushApprovedChanges(
		t.repoPath, repo, t.currentBranch, commitMessageFor(t.req.UserRequest))
	if prErr != nil {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "PR_MANUAL",
			"PR 은 못 열었지만 브랜치는 올라갔습니다", prURL, prErr.Error())
	} else {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "COMPLETED", "성공", prURL, "")
	}
	return RunResult{RepoName: repo, PRURL: prURL}, true, nil
}
