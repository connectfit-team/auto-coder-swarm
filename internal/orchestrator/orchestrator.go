package orchestrator

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func (o *SwarmOrchestrator) runBuildVerification(path string) error {
	var cmd *exec.Cmd
	if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
		cmd = exec.Command("/usr/local/go/bin/go", "build", "./...")
	}

	if cmd == nil { return nil }
	cmd.Dir = path
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("build failed: %v, output: %s", err, string(out))
	}
	return nil
}

func (o *SwarmOrchestrator) RunTask(ctx context.Context, userRequest string, repoLockFunc func(string) (bool, error)) (string, error) {
	taskID := fmt.Sprintf("T-%d", time.Now().UnixNano()%1000000)
	o.reportStatus(taskID, "INIT", fmt.Sprintf("Starting task: %s", userRequest))

	o.reportStatus(taskID, "ORACLE", "Requesting analysis...")
	analysis, err := o.insightClient.QueryOracle(ctx, userRequest)
	if err != nil {
		return "", fmt.Errorf("oracle query failed: %w", err)
	}
	o.reportStatus(taskID, "ORACLE", fmt.Sprintf("Context received (%d bytes)", len(analysis)))

	wsPath, err := o.wsMgr.CreateWorkspace()
	if err != nil {
		return "", fmt.Errorf("workspace setup failed: %w", err)
	}
	defer o.wsMgr.Cleanup(wsPath)

	var lastFeedback string
	var currentBranch string
	var targetRepo string
	maxRetries := 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		o.reportStatus(taskID, "PLANNING", fmt.Sprintf("Attempt %d...", attempt))
		
		input := analysis
		if lastFeedback != "" {
			input = fmt.Sprintf("ANALYSIS:\n%s\n\nPREVIOUS REVIEW FEEDBACK:\n%s", analysis, lastFeedback)
		}

		planRaw, err := o.planner.Process(ctx, input)
		if err != nil {
			return "", fmt.Errorf("planning failed: %w", err)
		}
		plan, err := o.planner.ParsePlan(planRaw)
		if err != nil {
			return "", fmt.Errorf("plan parsing failed: %w", err)
		}
		
		if attempt == 1 {
			targetRepo = plan.RepoName
			if repoLockFunc != nil {
				locked, err := repoLockFunc(targetRepo)
				if err != nil { return "", err }
				if !locked {
					return "", fmt.Errorf("repository %s is currently locked", targetRepo)
				}
			}
		}

		o.reportStatus(taskID, "PLANNING", fmt.Sprintf("Target Repo: %s", plan.RepoName))

		repoPath := filepath.Join(wsPath, "repo")
		if attempt == 1 {
			o.reportStatus(taskID, "GIT", "Creating Instant Sandbox (Hardlink Clone)...")
			if err := o.wsMgr.CloneFast(plan.RepoName, repoPath); err != nil {
				o.reportStatus(taskID, "GIT", "Fast clone failed, falling back to slow clone")
				repoURL := fmt.Sprintf("/home/cnf/projects/code-insight-engine/repos/%s", plan.RepoName)
				if err := o.gitMgr.Clone(repoURL, repoPath); err != nil {
					return "", fmt.Errorf("git clone failed: %w", err)
				}
			}
			currentBranch = fmt.Sprintf("swarm-fix-%d", time.Now().Unix())
			o.gitMgr.CreateBranch(repoPath, currentBranch)
		}

		for _, change := range plan.Changes {
			o.reportStatus(taskID, "CODER", fmt.Sprintf("Modifying %s", change.FilePath))
			fullPath := filepath.Join(repoPath, change.FilePath)
			_, err := o.coder.ModifyFile(ctx, fullPath, change.Instructions)
			if err != nil {
				return "", fmt.Errorf("coder failed on %s: %w", change.FilePath, err)
			}
		}

		isDocOnly := true
		for _, c := range plan.Changes {
			if !strings.HasSuffix(c.FilePath, ".md") && !strings.HasSuffix(c.FilePath, ".txt") {
				isDocOnly = false; break
			}
		}

		if !isDocOnly {
			o.reportStatus(taskID, "VERIFY", "Running build verification...")
			if err := o.runBuildVerification(repoPath); err != nil {
				o.reportStatus(taskID, "VERIFY", "BUILD FAILED")
				lastFeedback = fmt.Sprintf("BUILD FAILED: %v", err)
				if attempt < maxRetries {
					exec.Command("git", "-C", repoPath, "checkout", ".").Run()
				}
				continue
			}
		}

		o.reportStatus(taskID, "REVIEW", "Agent reviewing changes...")
		diffCmd := exec.Command("git", "-C", repoPath, "diff", "HEAD")
		diffOut, _ := diffCmd.CombinedOutput()
		reviewResp, err := o.reviewer.Process(ctx, string(diffOut))
		if err != nil {
			return "", fmt.Errorf("review failed: %w", err)
		}

		if !o.reviewer.IsApproved(reviewResp) {
			o.reportStatus(taskID, "REVIEW", "REJECTED")
			lastFeedback = reviewResp
			if attempt < maxRetries {
				exec.Command("git", "-C", repoPath, "checkout", ".").Run()
			}
			continue
		}

		o.reportStatus(taskID, "RISK", "Assessing risks...")
		riskResp, err := o.riskAssessor.Process(ctx, string(diffOut))
		if err != nil {
			return "", fmt.Errorf("risk assessment failed: %w", err)
		}

		if !o.riskAssessor.IsSafe(riskResp) {
			o.reportStatus(taskID, "RISK", "DANGER DETECTED")
			lastFeedback = riskResp
			if attempt < maxRetries {
				exec.Command("git", "-C", repoPath, "checkout", ".").Run()
			}
			continue
		}

		o.reportStatus(taskID, "SUCCESS", "Creating PR...")
		prURL, err := o.gitMgr.PushApprovedChanges(repoPath, currentBranch, "feat: automated code modification")
		if err != nil {
			return "", fmt.Errorf("failed to generate PR: %w", err)
		}
		o.reportStatus(taskID, "FINISH", fmt.Sprintf("PR: %s", prURL))
		return targetRepo, nil
	}

	return targetRepo, fmt.Errorf("failed after max retries")
}
