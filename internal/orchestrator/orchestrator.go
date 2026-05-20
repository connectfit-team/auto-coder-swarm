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
	"encoding/json"

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
	llm           model.LLM
}

type RunResult struct {
	RepoName        string
	PRURL           string
	WaitingApproval bool
	ChainTasks      []StatelessRequest
}

type StatelessRequest struct {
	UserRequest     string   `json:"user_request"`
	AnalysisContext string   `json:"analysis_context,omitempty"`
	TargetRepo      string   `json:"target_repo,omitempty"`
	TargetFiles     []string `json:"target_files,omitempty"`
	Constraints     []string `json:"constraints,omitempty"`
	Depth           int      `json:"depth"`
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
		llm:           llm,
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

func (o *SwarmOrchestrator) runBenchmark(path string) (string, error) {
	if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
		cmd := exec.Command("/usr/local/go/bin/go", "test", "-bench=.", "-benchmem", "./...")
		cmd.Dir = path
		out, err := cmd.CombinedOutput()
		if err != nil { return "", err }
		return string(out), nil
	}
	return "", nil
}

func (o *SwarmOrchestrator) trySelfHealing(taskID, path, errMsg string) bool {
	if strings.Contains(errMsg, "module") || strings.Contains(errMsg, "go.sum") {
		cmd := exec.Command("/usr/local/go/bin/go", "mod", "tidy")
		cmd.Dir = path
		return cmd.Run() == nil
	}
	return false
}

func (o *SwarmOrchestrator) detectChainTasks(ctx context.Context, taskID, repoName string, diff string, currentDepth int) []StatelessRequest {
	if currentDepth >= 2 { return nil }
	prompt := fmt.Sprintf("You are the MSA Dependency Architect.\n"+
		"Based on the following changes in [%s], identify other repositories that need to be updated.\n"+
		"Return JSON array: [{\"repo_name\": \"...\", \"user_request\": \"...\"}]\n\n"+
		"[Changes]\n%s", repoName, diff)
	resp, err := agent.CallLLM(ctx, o.llm, "Architect", prompt)
	if err != nil { return nil }
	var findings []struct {
		RepoName    string `json:"repo_name"`
		UserRequest string `json:"user_request"`
	}
	start := strings.Index(resp, "["); end := strings.LastIndex(resp, "]")
	if start == -1 || end == -1 { return nil }
	if err := json.Unmarshal([]byte(resp[start:end+1]), &findings); err != nil { return nil }
	var tasks []StatelessRequest
	for _, f := range findings {
		tasks = append(tasks, StatelessRequest{UserRequest: f.UserRequest, TargetRepo: f.RepoName, Depth: currentDepth + 1})
	}
	return tasks
}

func (o *SwarmOrchestrator) RunStatelessTask(ctx context.Context, req StatelessRequest, isApproved bool, repoLockFunc func(string) (bool, error)) (RunResult, error) {
	taskID := fmt.Sprintf("T-%d", time.Now().UnixNano()%1000000)
	o.reportStatus(taskID, "INIT", fmt.Sprintf("Task start: %s", req.UserRequest))

	analysis := req.AnalysisContext
	if analysis == "" {
		var err error
		analysis, err = o.insightClient.QueryOracle(ctx, req.UserRequest)
		if err != nil { return RunResult{}, err }
	}

	if len(analysis) > 3000 {
		compressed, _ := o.summarizer.Process(ctx, analysis)
		if compressed != "" { analysis = compressed }
	}

	wsPath, err := o.wsMgr.CreateWorkspace()
	if err != nil { return RunResult{}, err }
	defer o.wsMgr.Cleanup(wsPath)

	var lastFeedback, currentBranch, targetRepo, finalDiff string
	var preBench, postBench string
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
			preBench, _ = o.runBenchmark(repoPath)
			currentBranch = fmt.Sprintf("swarm-fix-%d", time.Now().Unix())
			o.gitMgr.CreateBranch(repoPath, currentBranch)
		}

		for _, change := range plan.Changes {
			o.coder.ModifyFile(ctx, filepath.Join(repoPath, change.FilePath), change.Instructions)
			ext := filepath.Ext(change.FilePath)
			if ext == ".go" || ext == ".dart" { o.coder.GenerateTestFile(ctx, filepath.Join(repoPath, change.FilePath)) }
		}

		isDocOnly := true
		for _, c := range plan.Changes {
			if !strings.HasSuffix(c.FilePath, ".md") { isDocOnly = false; break }
		}
		if !isDocOnly {
			if err := o.runBuildVerification(repoPath); err != nil {
				if o.trySelfHealing(taskID, repoPath, err.Error()) {
					if err = o.runBuildVerification(repoPath); err == nil { goto build_passed }
				}
				lastFeedback = fmt.Sprintf("BUILD FAILED: %v", err); exec.Command("git", "-C", repoPath, "checkout", ".").Run(); continue
			}
		}

	build_passed:
		if !isDocOnly && preBench != "" { postBench, _ = o.runBenchmark(repoPath) }

		diffCmd := exec.Command("git", "-C", repoPath, "diff", "HEAD")
		diffOut, _ := diffCmd.CombinedOutput()
		finalDiff = string(diffOut)

		o.reportStatus(taskID, "AUDIT", "Parallel Audits...")
		var reviewResp, riskResp string; var reviewErr, riskErr error; var wg sync.WaitGroup
		wg.Add(2)
		go func() { 
			defer wg.Done()
			reviewInput := fmt.Sprintf("DIFF:\n%s\n\nPRE-BENCHMARK:\n%s\n\nPOST-BENCHMARK:\n%s", finalDiff, preBench, postBench)
			reviewResp, reviewErr = o.reviewer.Process(ctx, reviewInput) 
		}()
		go func() { defer wg.Done(); riskResp, riskErr = o.riskAssessor.Process(ctx, finalDiff) }()
		wg.Wait()

		if reviewErr != nil || riskErr != nil { return RunResult{RepoName: targetRepo}, fmt.Errorf("audit failed") }

		if !o.reviewer.IsApproved(reviewResp) {
			lastFeedback = reviewResp; exec.Command("git", "-C", repoPath, "checkout", ".").Run(); continue
		}
		if !o.riskAssessor.IsSafe(riskResp) {
			lastFeedback = riskResp; exec.Command("git", "-C", repoPath, "checkout", ".").Run(); continue
		}

		if !isApproved {
			o.reportStatus(taskID, "WAIT", "Approval required.")
			return RunResult{RepoName: targetRepo, WaitingApproval: true}, nil
		}

		o.reportStatus(taskID, "SUCCESS", "Creating PR...")
		prURL, err := o.gitMgr.PushApprovedChanges(repoPath, targetRepo, currentBranch, "feat: automated modification")
		if err != nil { return RunResult{RepoName: targetRepo}, err }
		
		// Step 27: Self-Knowledge Feedback Loop
		o.reportStatus(taskID, "KNOWLEDGE", "Syncing modification summary to Oracle...")
		syncPrompt := fmt.Sprintf("You are the Knowledge Architect. Summarize the changes made in [%s] in 2-3 sentences and list core keywords for indexing.\n\n[Changes]\n%s", targetRepo, finalDiff)
		summary, _ := agent.CallLLM(ctx, o.llm, "Architect", syncPrompt)
		o.insightClient.UpdateKnowledge(ctx, targetRepo, summary, "automated-fix, swarm-agent")

		chainTasks := o.detectChainTasks(ctx, taskID, targetRepo, finalDiff, req.Depth)
		return RunResult{RepoName: targetRepo, PRURL: prURL, ChainTasks: chainTasks}, nil
	}
	return RunResult{RepoName: targetRepo}, fmt.Errorf("max retries")
}
