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
	linkDependencies(sourcePath, targetPath)
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

// 새 worktree 는 **의존성이 없다.**
//
// git worktree 는 추적되는 파일만 준다. node_modules 는 .gitignore 이므로
// 안 따라오고, 그러면 JS/TS 저장소는 무슨 코드를 쓰든 빌드가 실패한다:
//
//	> vite build
//	sh: 1: vite: not found
//
// 검증이 원천적으로 불가능하니 자가치유 세 번을 태우고 끝난다. 원본 클론에
// 한 번 설치해 두고(cms 기준 9초·649MB) worktree 마다 이어 붙인다.
// 복사하지 않는다 — 작업마다 649MB 를 쓰면 디스크가 남아나지 않는다.
func linkDependencies(sourcePath, targetPath string) {
	for _, dir := range []string{"node_modules", ".venv", "vendor"} {
		src := filepath.Join(sourcePath, dir)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		dst := filepath.Join(targetPath, dir)
		if _, err := os.Lstat(dst); err == nil {
			continue // worktree 가 이미 갖고 있으면 건드리지 않는다
		}
		if err := os.Symlink(src, dst); err != nil {
			log.Printf("[Workspace] %s 연결 실패: %v", dir, err)
			continue
		}
		log.Printf("[Workspace] %s 를 원본에서 이어 붙였다", dir)
	}

	// **생성물이 있어야 빌드가 된다.**
	//
	// SvelteKit 은 .svelte-kit/ 을 만들어야 tsconfig 가 풀리고, 그건 보통
	// npm install 의 훅이 한다. 의존성을 연결만 하면 그 단계를 건너뛴다.
	// 있으면 돌리고 없으면 그냥 넘어간다 — 다른 생태계를 망가뜨리지 않는다.
	if _, err := os.Stat(filepath.Join(targetPath, "svelte.config.js")); err == nil {
		run(targetPath, "npx", "svelte-kit", "sync")
	}
	if _, err := os.Stat(filepath.Join(targetPath, "package.json")); err == nil {
		run(targetPath, "npm", "run", "prepare", "--if-present")
	}
}

// run 은 준비 단계를 돌린다. 실패해도 멈추지 않는다 — 기준 빌드가 판정한다.
func run(dir, name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("[Workspace] %s 실패(계속 진행): %v · %s", name, err, firstLine(string(out)))
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 160 {
		s = s[:160]
	}
	return s
}
