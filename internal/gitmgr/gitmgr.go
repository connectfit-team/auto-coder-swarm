package gitmgr

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
)

type GitManager struct{}

func NewGitManager() *GitManager {
	return &GitManager{}
}

func (m *GitManager) Clone(repoURL, targetPath string) error {
	log.Printf("[GitMgr] Cloning repository from %s to %s", repoURL, targetPath)
	cmd := exec.Command("git", "clone", repoURL, targetPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("[GitMgr] [ERROR] Clone failed: %v, Output: %s", err, string(out))
		return fmt.Errorf("git clone failed: %v, output: %s", err, string(out))
	}
	return nil
}

func (m *GitManager) CreateBranch(path, branchName string) error {
	log.Printf("[GitMgr] [%s] Creating feature branch: %s", path, branchName)
	cmd := exec.Command("git", "-C", path, "checkout", "-b", branchName)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("[GitMgr] [ERROR] Branch creation failed: %v, Output: %s", err, string(out))
		return fmt.Errorf("failed to create branch: %v, output: %s", err, string(out))
	}
	return nil
}

func (m *GitManager) PushApprovedChanges(path, repoName, branchName, message string) (string, error) {
	log.Printf("[GitMgr] [%s] Finalizing changes for repo: %s, branch: %s", path, repoName, branchName)

	githubURL := fmt.Sprintf("https://github.com/connectfit-team/%s.git", repoName)
	log.Printf("[GitMgr] [%s] Setting remote URL to: %s", path, githubURL)
	remoteCmd := exec.Command("git", "-C", path, "remote", "set-url", "origin", githubURL)
	if out, err := remoteCmd.CombinedOutput(); err != nil {
		log.Printf("[GitMgr] [ERROR] Remote set-url failed: %v", err)
		return "", fmt.Errorf("failed to set remote URL: %v, output: %s", err, string(out))
	}

	log.Printf("[GitMgr] [%s] Staging and committing changes", path)
	if out, err := exec.Command("git", "-C", path, "add", ".").CombinedOutput(); err != nil {
		return "", fmt.Errorf("git add failed: %v, output: %s", err, string(out))
	}

	commitCmd := exec.Command("git", "-C", path, "commit", "-m", message)
	if out, err := commitCmd.CombinedOutput(); err != nil {
		log.Printf("[GitMgr] [ERROR] Commit failed (possibly no changes): %v", err)
		return "", fmt.Errorf("git commit failed: %v, output: %s", err, string(out))
	}
	
	log.Printf("[GitMgr] [%s] Pushing branch to GitHub", path)
	pushCmd := exec.Command("git", "-C", path, "push", "origin", branchName)
	if out, err := pushCmd.CombinedOutput(); err != nil {
		log.Printf("[GitMgr] [ERROR] Push failed: %v, Output: %s", err, string(out))
		return "", fmt.Errorf("git push failed: %v, output: %s", err, string(out))
	}

	log.Printf("[GitMgr] [%s] Creating Pull Request via gh CLI", path)
	prCmd := exec.Command("gh", "pr", "create", 
		"--title", message, 
		"--body", "This Pull Request was automatically generated and verified by the Auto-Coder Swarm.",
		"--head", branchName,
		"--repo", fmt.Sprintf("connectfit-team/%s", repoName))
	prCmd.Dir = path

	out, err := prCmd.CombinedOutput()
	if err != nil {
		log.Printf("[GitMgr] [ERROR] gh pr create failed: %v, Output: %s", err, string(out))
		return "", fmt.Errorf("gh pr create failed: %v, output: %s", err, string(out))
	}

	prURL := strings.TrimSpace(string(out))
	log.Printf("[GitMgr] [%s] PR created successfully: %s", path, prURL)
	return prURL, nil
}
