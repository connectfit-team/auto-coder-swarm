package workspace

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

// Manager handles ephemeral workspaces for code modification.
type Manager interface {
	CreateWorkspace() (string, error)
	Cleanup(path string) error
}

type LocalManager struct {
	baseDir string
}

func NewLocalManager(baseDir string) *LocalManager {
	if baseDir == "" {
		baseDir = os.TempDir()
	}
	return &LocalManager{baseDir: baseDir}
}

// CreateWorkspace creates a new isolated directory with a unique UUID.
func (m *LocalManager) CreateWorkspace() (string, error) {
	id := uuid.New().String()
	wsPath := filepath.Join(m.baseDir, fmt.Sprintf("swarm_ws_%s", id))

	if err := os.MkdirAll(wsPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create workspace: %w", err)
	}

	return wsPath, nil
}

// Cleanup removes the specified workspace directory and all its contents.
func (m *LocalManager) Cleanup(wsPath string) error {
	// Safety check: only delete directories that look like our workspaces
	if filepath.Base(wsPath)[:9] != "swarm_ws_" {
		return fmt.Errorf("refusing to delete non-swarm directory: %s", wsPath)
	}
	return os.RemoveAll(wsPath)
}
