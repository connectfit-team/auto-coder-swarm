package workspace

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/google/uuid"
)

type Manager interface {
	CreateWorkspace() (string, error)
	Cleanup(path string) error
	CloneFast(repoName, targetPath string) error
}

type LocalManager struct {
	baseDir    string
	masterRepos string
}

func NewLocalManager(baseDir, masterRepos string) *LocalManager {
	if baseDir == "" {
		baseDir = os.TempDir()
	}
	return &LocalManager{
		baseDir:     baseDir,
		masterRepos: masterRepos,
	}
}

func (m *LocalManager) CreateWorkspace() (string, error) {
	id := uuid.New().String()
	wsPath := filepath.Join(m.baseDir, fmt.Sprintf("swarm_ws_%s", id))

	if err := os.MkdirAll(wsPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create workspace: %w", err)
	}

	return wsPath, nil
}

func (m *LocalManager) CloneFast(repoName, targetPath string) error {
	sourcePath := filepath.Join(m.masterRepos, repoName)
	if _, err := os.Stat(sourcePath); err != nil {
		return fmt.Errorf("master repository not found for %s: %w", repoName, err)
	}

	cmd := exec.Command("cp", "-al", sourcePath, targetPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("hardlink copy failed: %v, output: %s", err, string(out))
	}

	return nil
}

func (m *LocalManager) Cleanup(wsPath string) error {
	if filepath.Base(wsPath)[:9] != "swarm_ws_" {
		return fmt.Errorf("refusing to delete non-swarm directory: %s", wsPath)
	}
	return os.RemoveAll(wsPath)
}
