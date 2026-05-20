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

	// 1. Initial Analysis
	analysis, err := o.insightClient.QueryOracle(ctx, userRequest)
	if err != nil {
		return fmt.Errorf("oracle query failed: %w", err)
	}

	// 2. Setup Workspace
	wsPath, err := o.wsMgr.CreateWorkspace()
	if err != nil {
		return fmt.Errorf("workspace setup failed: %w", err)
	}
	defer o.wsMgr.Cleanup(wsPath)

	// 3. Planning & Loop
	var lastFeedback string
	maxRetries := 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		log.Printf("[Orchestrator] --- Modification Attempt %d ---", attempt)
		
		input := analysis
		if lastFeedback != "" {
			input = fmt.Sprintf("ANALYSIS:\n%s\n\nPREVIOUS REVIEW FEEDBACK:\n%s", analysis, lastFeedback)
		}

		planRaw, err := o.planner.Process(ctx, input)
		if err != nil {
			return fmt.Errorf("planning failed on attempt %d: %w", attempt, err)
		}
		plan, err := o.planner.ParsePlan(planRaw)
		if err != nil {
			return fmt.Errorf("plan parsing failed on attempt %d: %w", attempt, err)
		}

		repoPath := filepath.Join(wsPath, "repo")
		
		// Clone only on first attempt
		if attempt == 1 {
			repoURL := fmt.Sprintf("/home/cnf/projects/code-insight-engine/repos/%s", plan.RepoName)
			if err := o.gitMgr.Clone(repoURL, repoPath); err != nil {
				return fmt.Errorf("git clone failed: %w", err)
			}
			branchName := fmt.Sprintf("swarm-fix-%d", time.Now().Unix())
			if err := o.gitMgr.CreateBranch(repoPath, branchName); err != nil {
				return fmt.Errorf("branch creation failed: %w", err)
			}
		}

		// Apply Changes
		for _, change := range plan.Changes {
			fullPath := filepath.Join(repoPath, change.FilePath)
			_, err := o.coder.ModifyFile(ctx, fullPath, change.Instructions)
			if err != nil {
				return fmt.Errorf("coder failed: %w", err)
			}
		}

		// Review
		diffCmd := exec.Command("git", "-C", repoPath, "diff", "HEAD")
		diffOut, _ := diffCmd.CombinedOutput()

		reviewResp, err := o.reviewer.Process(ctx, string(diffOut))
		if err != nil {
			return fmt.Errorf("review failed: %w", err)
		}

		if o.reviewer.IsApproved(reviewResp) {
			log.Println("[Orchestrator] Review APPROVED!")
			break
		}

		log.Printf("[Orchestrator] Review REJECTED on attempt %d. Feedback: %s", attempt, reviewResp)
		lastFeedback = reviewResp
		
		if attempt == maxRetries {
			return fmt.Errorf("failed to pass review after %d attempts: %s", maxRetries, lastFeedback)
		}
		
		// Reset changes for next attempt (simplified: git checkout .)
		exec.Command("git", "-C", repoPath, "checkout", ".").Run()
	}

	log.Println("[Orchestrator] Swarm process completed successfully.")
	return nil
}
