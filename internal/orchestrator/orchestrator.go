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
	riskAssessor  *agent.RiskAssessorAgent
}

func NewSwarmOrchestrator(ic *insightclient.Client, ws workspace.Manager, gm *gitmgr.GitManager, llm model.LLM) *SwarmOrchestrator {
	return &SwarmOrchestrator{
		insightClient: ic,
		wsMgr:         ws,
		gitMgr:        gm,
		planner:       agent.NewPlannerAgent(llm),
		coder:         agent.NewCoderAgent(llm),
		reviewer:      agent.NewReviewerAgent(llm),
		riskAssessor:  agent.NewRiskAssessorAgent(llm),
	}
}

func (o *SwarmOrchestrator) reportStatus(taskID, stage, message string) {
	timestamp := time.Now().Format("15:04:05")
	log.Printf("[%s] [%s] [%s] %s", taskID, timestamp, stage, message)
}

func (o *SwarmOrchestrator) RunTask(ctx context.Context, userRequest string) error {
	taskID := fmt.Sprintf("T-%d", time.Now().UnixNano()%1000000)
	o.reportStatus(taskID, "INIT", fmt.Sprintf("Starting task: %s", userRequest))

	o.reportStatus(taskID, "ORACLE", "Consulting Code-Insight Engine for context...")
	analysis, err := o.insightClient.QueryOracle(ctx, userRequest)
	if err != nil {
		return fmt.Errorf("oracle query failed: %w", err)
	}
	o.reportStatus(taskID, "ORACLE", fmt.Sprintf("Received context (%d bytes)", len(analysis)))

	o.reportStatus(taskID, "SANDBOX", "Creating isolated workspace...")
	wsPath, err := o.wsMgr.CreateWorkspace()
	if err != nil {
		return fmt.Errorf("workspace setup failed: %w", err)
	}
	defer func() {
		o.reportStatus(taskID, "CLEANUP", "Removing ephemeral workspace")
		o.wsMgr.Cleanup(wsPath)
	}()

	var lastFeedback string
	var currentBranch string
	maxRetries := 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		o.reportStatus(taskID, "PLANNING", fmt.Sprintf("Attempt %d: Drafting modification plan...", attempt))
		
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
		o.reportStatus(taskID, "PLANNING", fmt.Sprintf("Plan validated for repo: %s", plan.RepoName))

		repoPath := filepath.Join(wsPath, "repo")
		if attempt == 1 {
			o.reportStatus(taskID, "GIT", fmt.Sprintf("Cloning %s into sandbox...", plan.RepoName))
			repoURL := fmt.Sprintf("/home/cnf/projects/code-insight-engine/repos/%s", plan.RepoName)
			if err := o.gitMgr.Clone(repoURL, repoPath); err != nil {
				return fmt.Errorf("git clone failed: %w", err)
			}
			currentBranch = fmt.Sprintf("swarm-fix-%d", time.Now().Unix())
			o.gitMgr.CreateBranch(repoPath, currentBranch)
		}

		for _, change := range plan.Changes {
			o.reportStatus(taskID, "CODER", fmt.Sprintf("Modifying file: %s", change.FilePath))
			fullPath := filepath.Join(repoPath, change.FilePath)
			_, err := o.coder.ModifyFile(ctx, fullPath, change.Instructions)
			if err != nil {
				return fmt.Errorf("coder failed on %s: %w", change.FilePath, err)
			}
		}

		diffCmd := exec.Command("git", "-C", repoPath, "diff", "HEAD")
		diffOut, _ := diffCmd.CombinedOutput()
		diffStr := string(diffOut)

		o.reportStatus(taskID, "REVIEW", "Performing 1st pass: Quality & Convention Review...")
		reviewResp, err := o.reviewer.Process(ctx, diffStr)
		if err != nil {
			return fmt.Errorf("review failed: %w", err)
		}

		if !o.reviewer.IsApproved(reviewResp) {
			o.reportStatus(taskID, "REVIEW", fmt.Sprintf("REJECTED: %s", reviewResp))
			lastFeedback = reviewResp
			if attempt < maxRetries {
				exec.Command("git", "-C", repoPath, "checkout", ".").Run()
			}
			continue
		}

		o.reportStatus(taskID, "RISK", "Performing 2nd pass: System Impact & Risk Assessment...")
		riskResp, err := o.riskAssessor.Process(ctx, diffStr)
		if err != nil {
			return fmt.Errorf("risk assessment failed: %w", err)
		}

		if !o.riskAssessor.IsSafe(riskResp) {
			o.reportStatus(taskID, "RISK", fmt.Sprintf("DANGER DETECTED: %s", riskResp))
			lastFeedback = fmt.Sprintf("REVIEW WAS APPROVED BUT RISKS WERE DETECTED:\n%s", riskResp)
			if attempt < maxRetries {
				exec.Command("git", "-C", repoPath, "checkout", ".").Run()
			}
			continue
		}

		o.reportStatus(taskID, "SUCCESS", "All checks passed. Proceeding to PR generation.")
		prURL, err := o.gitMgr.PushApprovedChanges(repoPath, currentBranch, "feat: automated code modification with full audit")
		if err != nil {
			return fmt.Errorf("failed to generate PR: %w", err)
		}
		o.reportStatus(taskID, "FINISH", fmt.Sprintf("PR Link: %s", prURL))
		return nil
	}

	return fmt.Errorf("failed to pass all audits after %d attempts", maxRetries)
}
