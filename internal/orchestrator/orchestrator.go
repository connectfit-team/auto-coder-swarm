package orchestrator

import (
	"context"
	"fmt"
	"log"
	"os/exec"
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

	repoURL := fmt.Sprintf("/home/cnf/projects/code-insight-engine/repos/%s", plan.RepoName)
	repoPath := filepath.Join(wsPath, "repo")
	if err := o.gitMgr.Clone(repoURL, repoPath); err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	branchName := fmt.Sprintf("swarm-fix-%d", time.Now().Unix())
	if err := o.gitMgr.CreateBranch(repoPath, branchName); err != nil {
		return fmt.Errorf("branch creation failed: %w", err)
	}

	// Modification & Review Loop (Simple 1-pass for now)
	log.Println("[Orchestrator] Applying changes...")
	for _, change := range plan.Changes {
		fullPath := filepath.Join(repoPath, change.FilePath)
		_, err := o.coder.ModifyFile(ctx, fullPath, change.Instructions)
		if err != nil {
			return fmt.Errorf("coder failed: %w", err)
		}
	}

	// Step 6: Review
	log.Println("[Orchestrator] Step 6: Reviewing changes...")
	diffCmd := exec.Command("git", "-C", repoPath, "diff", "HEAD")
	diffOut, _ := diffCmd.CombinedOutput()

	reviewResp, err := o.reviewer.Process(ctx, string(diffOut))
	if err != nil {
		return fmt.Errorf("review failed: %w", err)
	}

	if !o.reviewer.IsApproved(reviewResp) {
		log.Printf("[Orchestrator] Review REJECTED: %s", reviewResp)
		return fmt.Errorf("changes rejected by reviewer: %s", reviewResp)
	}

	log.Println("[Orchestrator] Review APPROVED. Generating PR...")

	// Step 7: Push & PR Simulation (Final phase)
	// For local test, we just log the commit. In production, we'd call CommitAndPush.
	log.Printf("[Orchestrator] Successfully modified %d files and passed review.", len(plan.Changes))

	return nil
}
