package gitmgr

import (
	"fmt"
	"log"
	"os"
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

	statusCmd := exec.Command("git", "-C", path, "status", "--porcelain")
	statusOut, _ := statusCmd.CombinedOutput()
	if len(strings.TrimSpace(string(statusOut))) == 0 {
		log.Printf("[GitMgr] [%s] No changes detected. Skipping commit and PR.", path)
		return "No changes made", nil
	}

	commitCmd := exec.Command("git", "-C", path, "commit", "-m", message)
	if out, err := commitCmd.CombinedOutput(); err != nil {
		log.Printf("[GitMgr] [ERROR] Commit failed: %v", err)
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
		"--body", prBody(path),
		"--head", branchName,
		"--repo", fmt.Sprintf("connectfit-team/%s", repoName))
	prCmd.Dir = path
	prCmd.Env = os.Environ()

	out, err := prCmd.CombinedOutput()
	if err != nil {
		// **PR 을 못 열었다고 push 한 것까지 잃지 않는다.**
		//
		// gh 인증이 없으면(office 는 SSH 키로 push 하고 API 토큰은 없다)
		// 여기서 오류를 내고 끝났다. 그러면 브랜치는 올라가 있는데 사람은
		// 그 이름조차 못 듣는다. 한 번에 열 수 있는 주소를 돌려준다.
		compare := fmt.Sprintf("https://github.com/connectfit-team/%s/compare/%s?expand=1",
			repoName, branchName)
		log.Printf("[GitMgr] [WARN] PR 을 못 열었다 (%v). 브랜치는 올라갔다: %s", err, compare)
		return compare, fmt.Errorf("PR 생성 실패 — 브랜치는 push 됐다. 여기서 열어라: %s (원인: %s)",
			compare, strings.TrimSpace(string(out)))
	}

	prURL := strings.TrimSpace(string(out))
	log.Printf("[GitMgr] [%s] PR created successfully: %s", path, prURL)
	return prURL, nil
}

// prBody 는 무엇을 왜 바꿨는지 적는다.
//
// 예전에는 "automatically generated and verified" 한 줄뿐이라, 받는 사람이
// diff 를 처음부터 읽어야 했다. 바뀐 파일과 줄 수는 기계가 알고 있다.
func prBody(path string) string {
	var b strings.Builder
	b.WriteString("Auto-Coder Swarm 이 만든 변경입니다. 사람 검토가 필요합니다.\n\n")

	if out, err := exec.Command("git", "-C", path, "diff", "--stat", "HEAD~1").Output(); err == nil {
		if s := strings.TrimSpace(string(out)); s != "" {
			b.WriteString("## 바뀐 것\n\n```\n" + s + "\n```\n\n")
		}
	}
	if out, err := exec.Command("git", "-C", path, "log", "-1", "--format=%B").Output(); err == nil {
		if s := strings.TrimSpace(string(out)); s != "" {
			b.WriteString("## 커밋 메시지\n\n" + s + "\n\n")
		}
	}
	b.WriteString("---\n검토 시 확인할 것: 요청한 것만 바뀌었는지, 경계값·조건식이 맞는지.\n")
	return b.String()
}
