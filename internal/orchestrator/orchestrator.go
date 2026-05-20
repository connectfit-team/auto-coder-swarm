package orchestrator

import (
	"context"
	"fmt"
	"log"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
	"github.com/connectfit-team/auto-coder-swarm/internal/insightclient"
	"google.golang.org/adk/model"
)

type SwarmOrchestrator struct {
	insightClient *insightclient.Client
	planner       *agent.PlannerAgent
	coder         *agent.CoderAgent
	reviewer      *agent.ReviewerAgent
}

func NewSwarmOrchestrator(ic *insightclient.Client, llm model.LLM) *SwarmOrchestrator {
	return &SwarmOrchestrator{
		insightClient: ic,
		planner:       agent.NewPlannerAgent(llm),
		coder:         agent.NewCoderAgent(llm),
		reviewer:      agent.NewReviewerAgent(llm),
	}
}

func (o *SwarmOrchestrator) RunTask(ctx context.Context, userRequest string) error {
	log.Printf("[Orchestrator] Starting task: %s", userRequest)

	log.Println("[Orchestrator] Step 1: Consulting the Oracle...")
	analysis, err := o.insightClient.QueryOracle(ctx, userRequest)
	if err != nil {
		return fmt.Errorf("failed to get analysis from oracle: %w", err)
	}
	log.Println("[Orchestrator] Oracle analysis received.")

	log.Println("[Orchestrator] Step 2: Planning modifications...")
	planRaw, err := o.planner.Process(ctx, analysis)
	if err != nil {
		return fmt.Errorf("planning failed: %w", err)
	}

	plan, err := o.planner.ParsePlan(planRaw)
	if err != nil {
		log.Printf("[Orchestrator] Warning: Initial JSON parse failed, attempting one recovery: %v", err)
		// Potential recovery logic can be added here
		return fmt.Errorf("failed to parse a valid plan: %w", err)
	}
	log.Printf("[Orchestrator] Plan validated! Target Repo: %s, Files: %d", plan.RepoName, len(plan.Changes))

	return nil
}
