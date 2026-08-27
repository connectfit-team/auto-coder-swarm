package orchestrator

import (
	"context"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
	"github.com/connectfit-team/auto-coder-swarm/internal/healing"
	"github.com/connectfit-team/auto-coder-swarm/internal/voter"
	"google.golang.org/adk/model"
)

type taskContext struct {
	ctx           context.Context
	taskID        string
	req           StatelessRequest
	isApproved    bool
	repoLockFunc  func(string) (bool, error)
	orchestrator  *SwarmOrchestrator
	primaryLLM    model.LLM
	voter         *voter.MultiModelVoter
	planner       *agent.PlannerAgent
	coder         *agent.CoderAgent
	reviewer      *agent.ReviewerAgent
	critic        *agent.CriticAgent
	healer        *healing.HealerAgent
	analysis      string
	ckhKnowledge  string     // Corporate Knowledge from CKH
	skills        []SkillDoc // CIE 가 준 팀의 작업 절차
	wsPath        string
	repoPath      string
	targetRepo    string
	currentBranch string
	lastFeedback  string
	finalDiff     string
	preBench      string
	postBench     string
	meta          ProjectMetadata
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
