package workspace

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

type Manager interface {
	CreateWorkspace() (string, error)
	Cleanup(path string) error
	CreateWorktree(repoName, targetPath, branchName string) error
}

type LocalManager struct {
	baseDir     string
	masterRepos string
}

func NewLocalManager(baseDir, masterRepos string) *LocalManager {
	if baseDir == "" {
		baseDir = os.TempDir()
	}
	log.Printf("[Workspace] Initialized LocalManager with baseDir: %s, masterRepos: %s", baseDir, masterRepos)
	return &LocalManager{
		baseDir:     baseDir,
		masterRepos: masterRepos,
	}
}

func (m *LocalManager) CreateWorkspace() (string, error) {
	id := uuid.New().String()
	wsPath := filepath.Join(m.baseDir, fmt.Sprintf("swarm_ws_%s", id))

	log.Printf("[Workspace] Creating new isolated directory: %s", wsPath)
	if err := os.MkdirAll(wsPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create workspace directory %s: %w", wsPath, err)
	}

	return wsPath, nil
}

func (m *LocalManager) CreateWorktree(repoName, targetPath, branchName string) error {
	sourcePath := filepath.Join(m.masterRepos, repoName)
	log.Printf("[Workspace] Creating Git Worktree for %s at %s (branch: %s)", repoName, targetPath, branchName)

	if _, err := os.Stat(sourcePath); err != nil {
		return fmt.Errorf("master repository not found: %s", sourcePath)
	}

	// 1. Prune stale worktrees first to ensure clean state
	exec.Command("git", "-C", sourcePath, "worktree", "prune").Run()

	// 2. Add worktree
	cmd := exec.Command("git", "-C", sourcePath, "worktree", "add", "-b", branchName, targetPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		if strings.Contains(string(out), "already exists") {
			cmd = exec.Command("git", "-C", sourcePath, "worktree", "add", targetPath, branchName)
			if out, err = cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("git worktree add failed: %v, output: %s", err, string(out))
			}
		} else {
			return fmt.Errorf("git worktree add failed: %v, output: %s", err, string(out))
		}
	}

	log.Printf("[Workspace] Worktree created successfully for %s", repoName)
	return nil
}

func (m *LocalManager) Cleanup(wsPath string) error {
	base := filepath.Base(wsPath)
	if !strings.HasPrefix(base, "swarm_ws_") {
		log.Printf("[Workspace] [WARNING] Refusing to delete unauthorized directory: %s", wsPath)
		return fmt.Errorf("refusing to delete non-swarm directory: %s", wsPath)
	}

	repoPath := filepath.Join(wsPath, "repo")
	if _, err := os.Stat(repoPath); err == nil {
		log.Printf("[Workspace] Removing Git Worktree: %s", repoPath)
		cmd := exec.Command("git", "worktree", "remove", "--force", repoPath)
		if out, err := cmd.CombinedOutput(); err != nil {
			log.Printf("[Workspace] [WARNING] Worktree remove failed: %v, Output: %s", err, string(out))
		}
	}

	log.Printf("[Workspace] Cleaning up directory: %s", wsPath)
	return os.RemoveAll(wsPath)
}
