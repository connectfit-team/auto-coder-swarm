package gitmgr

import (
	"fmt"
	"os/exec"
)

// GitManager handles repository operations in isolated workspaces.
type GitManager struct{}

func NewGitManager() *GitManager {
	return &GitManager{}
}

// Clone repository into the target workspace path.
func (m *GitManager) Clone(repoURL, targetPath string) error {
	cmd := exec.Command("git", "clone", repoURL, targetPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone failed: %v, output: %s", err, string(out))
	}
	return nil
}

// CreateBranch creates a new branch in the specified path.
func (m *GitManager) CreateBranch(path, branchName string) error {
	cmd := exec.Command("git", "-C", path, "checkout", "-b", branchName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create branch: %v, output: %s", err, string(out))
	}
	return nil
}

// CommitAndPush commits changes and pushes to a new remote branch.
func (m *GitManager) CommitAndPush(path, message, branchName string) error {
	// 1. Add
	if out, err := exec.Command("git", "-C", path, "add", ".").CombinedOutput(); err != nil {
		return fmt.Errorf("git add failed: %v, output: %s", err, string(out))
	}
	// 2. Commit
	if out, err := exec.Command("git", "-C", path, "commit", "-m", message).CombinedOutput(); err != nil {
		return fmt.Errorf("git commit failed: %v, output: %s", err, string(out))
	}
	// 3. Push
	if out, err := exec.Command("git", "-C", path, "push", "origin", branchName).CombinedOutput(); err != nil {
		return fmt.Errorf("git push failed: %v, output: %s", err, string(out))
	}
	return nil
}
