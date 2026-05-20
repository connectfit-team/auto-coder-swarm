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
	"sync"

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
	summarizer    *agent.SummarizerAgent
}

type RunResult struct {
	RepoName string
	PRURL    string
}

type StatelessRequest struct {
	UserRequest     string   `json:"user_request"`
	AnalysisContext string   `json:"analysis_context,omitempty"`
	TargetRepo      string   `json:"target_repo,omitempty"`
	TargetFiles     []string `json:"target_files,omitempty"`
	Constraints     []string `json:"constraints,omitempty"`
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
		summarizer:    agent.NewSummarizerAgent(llm),
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
	} else if _, err := os.Stat(filepath.Join(path, "pubspec.yaml")); err == nil {
		cmd = exec.Command("flutter", "analyze")
	}

	if cmd == nil { return nil }
	cmd.Dir = path
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("build failed: %v, output: %s", err, string(out))
	}
	return nil
}

// trySelfHealing analyzes build error and tries to fix environment.
func (o *SwarmOrchestrator) trySelfHealing(taskID, path, errMsg string) bool {
	o.reportStatus(taskID, "HEAL", "Analyzing build error for self-healing...")
	
	// Case 1: Missing Go modules
	if strings.Contains(errMsg, "no required module provides package") || strings.Contains(errMsg, "missing go.sum entry") {
		o.reportStatus(taskID, "HEAL", "Detected missing Go dependencies. Running go mod tidy...")
		cmd := exec.Command("/usr/local/go/bin/go", "mod", "tidy")
		cmd.Dir = path
		if err := cmd.Run(); err == nil {
			o.reportStatus(taskID, "HEAL", "go mod tidy successful.")
			return true
		}
	}

	// Case 2: Flutter/Dart dependency issues
	if strings.Contains(errMsg, "pub get") || strings.Contains(errMsg, "Target of URI doesn't exist") {
		o.reportStatus(taskID, "HEAL", "Detected missing Flutter dependencies. Running flutter pub get...")
		cmd := exec.Command("flutter", "pub", "get")
		cmd.Dir = path
		if err := cmd.Run(); err == nil {
			o.reportStatus(taskID, "HEAL", "flutter pub get successful.")
			return true
		}
	}

	o.reportStatus(taskID, "HEAL", "No automated fix available for this error.")
	return false
}

func (o *SwarmOrchestrator) RunStatelessTask(ctx context.Context, req StatelessRequest, repoLockFunc func(string) (bool, error)) (RunResult, error) {
	taskID := fmt.Sprintf("T-%d", time.Now().UnixNano()%1000000)
	o.reportStatus(taskID, "INIT", fmt.Sprintf("Stateless task start: %s", req.UserRequest))

	analysis := req.AnalysisContext
	if analysis == "" {
		o.reportStatus(taskID, "ORACLE", "Requesting analysis...")
		var err error
		analysis, err = o.insightClient.QueryOracle(ctx, req.UserRequest)
		if err != nil { return RunResult{}, err }
	}

	if len(analysis) > 3000 {
		o.reportStatus(taskID, "OPTIMIZE", "Analysis context too large. Compressing...")
		compressed, err := o.summarizer.Process(ctx, analysis)
		if err == nil {
			analysis = compressed
			o.reportStatus(taskID, "OPTIMIZE", fmt.Sprintf("Context compressed (%d bytes)", len(analysis)))
		}
	}

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
		if len(req.Constraints) > 0 {
			input += "\n\nCONSTRAINTS:\n- " + strings.Join(req.Constraints, "\n- ")
		}

		planRaw, err := o.planner.Process(ctx, input)
		if err != nil { return RunResult{}, err }
		plan, err := o.planner.ParsePlan(planRaw)
		if err != nil { return RunResult{}, err }
		
		if attempt == 1 {
			targetRepo = plan.RepoName
			if req.TargetRepo != "" { targetRepo = req.TargetRepo }
			if repoLockFunc != nil {
				locked, err := repoLockFunc(targetRepo)
				if err != nil { return RunResult{RepoName: targetRepo}, err }
				if !locked { return RunResult{RepoName: targetRepo}, fmt.Errorf("repo %s locked", targetRepo) }
			}
		}

		repoPath := filepath.Join(wsPath, "repo")
		if attempt == 1 {
			if err := o.wsMgr.CloneFast(targetRepo, repoPath); err != nil {
				repoURL := fmt.Sprintf("/home/cnf/projects/code-insight-engine/repos/%s", targetRepo)
				o.gitMgr.Clone(repoURL, repoPath)
			}
			currentBranch = fmt.Sprintf("swarm-fix-%d", time.Now().Unix())
			o.gitMgr.CreateBranch(repoPath, currentBranch)
		}

		for _, change := range plan.Changes {
			o.reportStatus(taskID, "CODER", fmt.Sprintf("Modifying %s", change.FilePath))
			fullPath := filepath.Join(repoPath, change.FilePath)
			_, err := o.coder.ModifyFile(ctx, fullPath, change.Instructions)
			if err != nil {
				return RunResult{RepoName: targetRepo}, fmt.Errorf("coder failed on %s: %w", change.FilePath, err)
			}

			ext := filepath.Ext(change.FilePath)
			if ext == ".go" || ext == ".dart" || ext == ".ts" {
				o.reportStatus(taskID, "TESTER", fmt.Sprintf("Generating unit test for %s", change.FilePath))
				_, _ = o.coder.GenerateTestFile(ctx, fullPath)
			}
		}

		isDocOnly := true
		for _, c := range plan.Changes {
			if !strings.HasSuffix(c.FilePath, ".md") { isDocOnly = false; break }
		}
		if !isDocOnly {
			o.reportStatus(taskID, "VERIFY", "Running build verification...")
			if err := o.runBuildVerification(repoPath); err != nil {
				// Step 24: Self-Healing Attempt
				if o.trySelfHealing(taskID, repoPath, err.Error()) {
					o.reportStatus(taskID, "VERIFY", "Retrying build after self-healing...")
					if err = o.runBuildVerification(repoPath); err == nil {
						o.reportStatus(taskID, "VERIFY", "Build FIXED by self-healing.")
						goto build_passed
					}
				}

				o.reportStatus(taskID, "VERIFY", "BUILD FAILED")
				lastFeedback = fmt.Sprintf("BUILD FAILED: %v", err)
				if attempt < maxRetries {
					exec.Command("git", "-C", repoPath, "checkout", ".").Run()
				}
				continue
			}
		}

	build_passed:
		diffCmd := exec.Command("git", "-C", repoPath, "diff", "HEAD")
		diffOut, _ := diffCmd.CombinedOutput()
		diffStr := string(diffOut)

		o.reportStatus(taskID, "AUDIT", "Parallel Audits...")
		var reviewResp, riskResp string; var reviewErr, riskErr error; var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); reviewResp, reviewErr = o.reviewer.Process(ctx, diffStr) }()
		go func() { defer wg.Done(); riskResp, riskErr = o.riskAssessor.Process(ctx, diffStr) }()
		wg.Wait()

		if reviewErr != nil || riskErr != nil { return RunResult{RepoName: targetRepo}, fmt.Errorf("audit failed") }

		if !o.reviewer.IsApproved(reviewResp) {
			lastFeedback = reviewResp; exec.Command("git", "-C", repoPath, "checkout", ".").Run(); continue
		}
		if !o.riskAssessor.IsSafe(riskResp) {
			lastFeedback = riskResp; exec.Command("git", "-C", repoPath, "checkout", ".").Run(); continue
		}

		o.reportStatus(taskID, "SUCCESS", "Creating PR...")
		prURL, err := o.gitMgr.PushApprovedChanges(repoPath, targetRepo, currentBranch, "feat: automated modification with auto-tests")
		if err != nil { return RunResult{RepoName: targetRepo}, err }
		return RunResult{RepoName: targetRepo, PRURL: prURL}, nil
	}
	return RunResult{RepoName: targetRepo}, fmt.Errorf("max retries")
}
