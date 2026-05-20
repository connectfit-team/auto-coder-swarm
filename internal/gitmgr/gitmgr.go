package gitmgr

import (
	"fmt"
	"os/exec"
	"strings"
)

type GitManager struct{}

func NewGitManager() *GitManager {
	return &GitManager{}
}

func (m *GitManager) Clone(repoURL, targetPath string) error {
	cmd := exec.Command("git", "clone", repoURL, targetPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone failed: %v, output: %s", err, string(out))
	}
	return nil
}

func (m *GitManager) CreateBranch(path, branchName string) error {
	cmd := exec.Command("git", "-C", path, "checkout", "-b", branchName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create branch: %v, output: %s", err, string(out))
	}
	return nil
}

func (m *GitManager) PushApprovedChanges(path, branchName, message string) (string, error) {
	if out, err := exec.Command("git", "-C", path, "add", ".").CombinedOutput(); err != nil {
		return "", fmt.Errorf("git add failed: %v, output: %s", err, string(out))
	}

	commitCmd := exec.Command("git", "-C", path, "commit", "-m", message)
	if out, err := commitCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git commit failed: %v, output: %s", err, string(out))
	}
	
	pushCmd := exec.Command("git", "-C", path, "push", "origin", branchName)
	if out, err := pushCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git push failed: %v, output: %s", err, string(out))
	}

	prCmd := exec.Command("gh", "pr", "create", 
		"--title", message, 
		"--body", "This Pull Request was automatically generated and verified by the Auto-Coder Swarm.",
		"--head", branchName)
	prCmd.Dir = path

	out, err := prCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gh pr create failed: %v, output: %s", err, string(out))
	}

	return strings.TrimSpace(string(out)), nil
}
