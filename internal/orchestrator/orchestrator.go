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

type RunResult struct {
	RepoName string
	PRURL    string
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

func (o *SwarmOrchestrator) RunTask(ctx context.Context, userRequest string, repoLockFunc func(string) (bool, error)) (RunResult, error) {
	taskID := fmt.Sprintf("T-%d", time.Now().UnixNano()%1000000)
	o.reportStatus(taskID, "INIT", fmt.Sprintf("Starting task: %s", userRequest))

	analysis, err := o.insightClient.QueryOracle(ctx, userRequest)
	if err != nil { return RunResult{}, err }

	wsPath, err := o.wsMgr.CreateWorkspace()
	if err != nil { return RunResult{}, err }
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
		if err != nil { return RunResult{}, err }
		plan, err := o.planner.ParsePlan(planRaw)
		if err != nil { return RunResult{}, err }
		
		if attempt == 1 {
			targetRepo = plan.RepoName
			if repoLockFunc != nil {
				locked, err := repoLockFunc(targetRepo)
				if err != nil { return RunResult{RepoName: targetRepo}, err }
				if !locked { return RunResult{RepoName: targetRepo}, fmt.Errorf("repo %s locked", targetRepo) }
			}
		}

		repoPath := filepath.Join(wsPath, "repo")
		if attempt == 1 {
			if err := o.wsMgr.CloneFast(plan.RepoName, repoPath); err != nil {
				repoURL := fmt.Sprintf("/home/cnf/projects/code-insight-engine/repos/%s", plan.RepoName)
				o.gitMgr.Clone(repoURL, repoPath)
			}
			currentBranch = fmt.Sprintf("swarm-fix-%d", time.Now().Unix())
			o.gitMgr.CreateBranch(repoPath, currentBranch)
		}

		for _, change := range plan.Changes {
			o.coder.ModifyFile(ctx, filepath.Join(repoPath, change.FilePath), change.Instructions)
		}

		isDocOnly := true
		for _, c := range plan.Changes {
			if !strings.HasSuffix(c.FilePath, ".md") { isDocOnly = false; break }
		}
		if !isDocOnly {
			if err := o.runBuildVerification(repoPath); err != nil {
				lastFeedback = fmt.Sprintf("BUILD FAILED: %v", err); exec.Command("git", "-C", repoPath, "checkout", ".").Run(); continue
			}
		}

		diffCmd := exec.Command("git", "-C", repoPath, "diff", "HEAD")
		diffOut, _ := diffCmd.CombinedOutput()
		reviewResp, _ := o.reviewer.Process(ctx, string(diffOut))
		if !o.reviewer.IsApproved(reviewResp) {
			lastFeedback = reviewResp; exec.Command("git", "-C", repoPath, "checkout", ".").Run(); continue
		}

		riskResp, _ := o.riskAssessor.Process(ctx, string(diffOut))
		if !o.riskAssessor.IsSafe(riskResp) {
			lastFeedback = riskResp; exec.Command("git", "-C", repoPath, "checkout", ".").Run(); continue
		}

		o.reportStatus(taskID, "SUCCESS", "Creating PR...")
		prURL, err := o.gitMgr.PushApprovedChanges(repoPath, targetRepo, currentBranch, "feat: automated modification")
		if err != nil { return RunResult{RepoName: targetRepo}, err }
		return RunResult{RepoName: targetRepo, PRURL: prURL}, nil
	}
	return RunResult{RepoName: targetRepo}, fmt.Errorf("max retries")
}
