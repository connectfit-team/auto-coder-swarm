package orchestrator

import (
	"context"
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
	"github.com/connectfit-team/auto-coder-swarm/internal/healing"
	"github.com/connectfit-team/auto-coder-swarm/internal/voter"
	"google.golang.org/adk/model"
)

type taskContext struct {
	ctx          context.Context
	taskID       string
	req          StatelessRequest
	isApproved   bool
	repoLockFunc func(string) (bool, error)
	// 계획이 없는 파일을 가리켰을 때 되돌려 줄 "실제로 있는 파일" 목록.
	candidateHint string
	// 전략 단계가 짚은 고칠 파일. 계획이 이걸 무시하면 엉뚱한 데를 고친다.
	actionablePath []string
	// 분석이 원인과 고칠 값까지 짚어 준 계획인가. 그렇다면 검토자의 반대는
	// 자문으로만 본다 — 근거는 분석 쪽에 있다.
	planFromAnalysis bool
	// 분석이 짚었지만 고칠 것이 없던 파일. 다음 시도에서는 건너뛴다.
	deadPaths []string
	// 반려로 되돌리기 직전의 고침. 시도를 다 쓰면 이것을 사람에게 넘긴다.
	lastRejectedDiff string
	orchestrator     *SwarmOrchestrator
	primaryLLM       model.LLM
	voter            *voter.MultiModelVoter
	planner          *agent.PlannerAgent
	coder            *agent.CoderAgent
	reviewer         *agent.ReviewerAgent
	critic           *agent.CriticAgent
	healer           *healing.HealerAgent
	analysis         string
	ckhKnowledge     string     // Corporate Knowledge from CKH
	skills           []SkillDoc // CIE 가 준 팀의 작업 절차
	wsPath           string
	repoPath         string
	targetRepo       string
	currentBranch    string
	lastFeedback     string
	finalDiff        string
	preBench         string
	postBench        string
	meta             ProjectMetadata
}

func (o *SwarmOrchestrator) newTaskContext(ctx context.Context, taskID string, req StatelessRequest, isApproved bool, repoLockFunc func(string) (bool, error)) *taskContext {
	primaryLLM, voterLLMs := o.loadModels()
	return &taskContext{
		ctx:          ctx,
		taskID:       taskID,
		req:          req,
		isApproved:   isApproved,
		repoLockFunc: repoLockFunc,
		orchestrator: o,
		primaryLLM:   primaryLLM,
		voter:        voter.NewMultiModelVoter(voterLLMs...),
		planner:      agent.NewPlannerAgent(primaryLLM),
		coder:        agent.NewCoderAgent(primaryLLM),
		reviewer:     agent.NewReviewerAgent(primaryLLM),
		critic:       agent.NewCriticAgent(primaryLLM),
		healer:       healing.NewHealerAgent(primaryLLM),
		analysis:     req.AnalysisContext,
	}
}

// keepRejectedDiff 는 되돌리기 전에 무엇을 고쳤는지 남긴다.
//
// 반려하면 작업공간을 git checkout 으로 되돌린다. 그러면 세 번째 시도가
// 반려됐을 때 넘겨줄 것이 남지 않아, 고친 것과 그 논쟁이 함께 사라진다.
// 사람이 보면 5초면 판정할 일이다.
func (t *taskContext) keepRejectedDiff() {
	if d := strings.TrimSpace(t.finalDiff); d != "" {
		t.lastRejectedDiff = t.finalDiff
	}
}
