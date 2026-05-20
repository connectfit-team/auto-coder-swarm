package gitmgr

import (
	"fmt"
	"log"
	"os/exec"
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
	exec.Command("git", "-C", path, "add", ".").Run()
	cmd := exec.Command("git", "-C", path, "commit", "-m", message)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("commit failed: %v, output: %s", err, string(out))
	}
	
	log.Printf("[GitManager] SIMULATION: git push origin %s", branchName)
	
	prURL := fmt.Sprintf("https://github.com/connectfit-team/simulated-pr/%s", branchName)
	return prURL, nil
}
