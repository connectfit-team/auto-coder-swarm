package workspace

import (
	"fmt"
	"log"
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

func (m *LocalManager) CloneFast(repoName, targetPath string) error {
	sourcePath := filepath.Join(m.masterRepos, repoName)
	log.Printf("[Workspace] Attempting fast clone from %s to %s", sourcePath, targetPath)
	
	if _, err := os.Stat(sourcePath); err != nil {
		log.Printf("[Workspace] [ERROR] Master repository not found: %s", sourcePath)
		return fmt.Errorf("master repository not found for %s: %w", repoName, err)
	}

	cmd := exec.Command("cp", "-al", sourcePath, targetPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("[Workspace] [ERROR] Hardlink copy failed: %v, Output: %s", err, string(out))
		return fmt.Errorf("hardlink copy failed: %v, output: %s", err, string(out))
	}

	log.Printf("[Workspace] Fast clone completed successfully for %s", repoName)
	return nil
}

func (m *LocalManager) Cleanup(wsPath string) error {
	if filepath.Base(wsPath)[:9] != "swarm_ws_" {
		log.Printf("[Workspace] [WARNING] Refusing to delete unauthorized directory: %s", wsPath)
		return fmt.Errorf("refusing to delete non-swarm directory: %s", wsPath)
	}
	log.Printf("[Workspace] Cleaning up directory: %s", wsPath)
	return os.RemoveAll(wsPath)
}
