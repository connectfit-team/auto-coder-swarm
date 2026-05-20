package orchestrator

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
	"github.com/connectfit-team/auto-coder-swarm/internal/gitmgr"
	"github.com/connectfit-team/auto-coder-swarm/internal/insightclient"
	"github.com/connectfit-team/auto-coder-swarm/internal/workspace"
	"google.golang.org/adk/model"
)

type SwarmOrchestrator struct {
	insightClient *insightclient.Client
	wsMgr         workspace.Manager
	gitMgr        *gitmgr.GitManager
	planner       *agent.PlannerAgent
	coder         *agent.CoderAgent
	reviewer      *agent.ReviewerAgent
}

func NewSwarmOrchestrator(ic *insightclient.Client, ws workspace.Manager, gm *gitmgr.GitManager, llm model.LLM) *SwarmOrchestrator {
	return &SwarmOrchestrator{
		insightClient: ic,
		wsMgr:         ws,
		gitMgr:        gm,
		planner:       agent.NewPlannerAgent(llm),
		coder:         agent.NewCoderAgent(llm),
		reviewer:      agent.NewReviewerAgent(llm),
	}
}

func (o *SwarmOrchestrator) RunTask(ctx context.Context, userRequest string) error {
	log.Printf("[Orchestrator] Starting task: %s", userRequest)

	analysis, err := o.insightClient.QueryOracle(ctx, userRequest)
	if err != nil {
		return fmt.Errorf("oracle query failed: %w", err)
	}

	planRaw, err := o.planner.Process(ctx, analysis)
	if err != nil {
		return fmt.Errorf("planning failed: %w", err)
	}
	plan, err := o.planner.ParsePlan(planRaw)
	if err != nil {
		return fmt.Errorf("plan parsing failed: %w", err)
	}
	log.Printf("[Orchestrator] Plan validated for repo: %s", plan.RepoName)

	wsPath, err := o.wsMgr.CreateWorkspace()
	if err != nil {
		return fmt.Errorf("workspace setup failed: %w", err)
	}
	defer o.wsMgr.Cleanup(wsPath)
	log.Printf("[Orchestrator] Isolated workspace ready: %s", wsPath)

	repoURL := fmt.Sprintf("/home/cnf/projects/code-insight-engine/repos/%s", plan.RepoName)
	repoPath := filepath.Join(wsPath, "repo")
	if err := o.gitMgr.Clone(repoURL, repoPath); err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	branchName := fmt.Sprintf("swarm-fix-%d", time.Now().Unix())
	if err := o.gitMgr.CreateBranch(repoPath, branchName); err != nil {
		return fmt.Errorf("branch creation failed: %w", err)
	}

	for _, change := range plan.Changes {
		fullPath := filepath.Join(repoPath, change.FilePath)
		log.Printf("[Orchestrator] Coder modifying file: %s", change.FilePath)
		_, err := o.coder.ModifyFile(ctx, fullPath, change.Instructions)
		if err != nil {
			return fmt.Errorf("coder failed on %s: %w", change.FilePath, err)
		}
	}

	log.Println("[Orchestrator] All changes applied in sandbox. Ready for Review.")
	return nil
}
