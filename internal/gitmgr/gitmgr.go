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

// PushOptions 는 PR 을 어떻게 열지 정한다. 빈 값이면 예전과 같이 연다.
type PushOptions struct {
	// 제목. 비면 커밋 메시지 첫 줄을 쓴다.
	Title string
	// 본문 맨 앞에 덧붙일 글. 다른 PR 이 먼저 머지돼야 한다는 안내 같은 것.
	BodyLead string
	// 초안으로 연다. 먼저 머지될 것이 있어 아직 빌드되지 않는 PR 에 쓴다.
	Draft bool
}

func (m *GitManager) PushApprovedChanges(path, repoName, branchName, message string) (string, error) {
	return m.PushApprovedChangesOpt(path, repoName, branchName, message, PushOptions{})
}

func (m *GitManager) PushApprovedChangesOpt(path, repoName, branchName, message string, opt PushOptions) (string, error) {
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
	title := opt.Title
	if title == "" {
		// 여러 줄을 통째로 넘기면 제목에 본문이 딸려 들어간다.
		title = firstLine(message)
	}
	args := []string{"pr", "create",
		"--title", title,
		"--body", opt.BodyLead + prBody(path),
		"--head", branchName,
		"--repo", fmt.Sprintf("connectfit-team/%s", repoName)}
	out, err := runPRCreate(path, args, opt.Draft)
	if err != nil {
		// PR 을 못 열어도 push 한 것은 잃지 않는다. gh 인증이 없을 수 있고
		// (office 는 SSH 로 push 한다), 그러면 브랜치만 올라간 채 이름조차
		// 전해지지 않는다. 바로 열 수 있는 주소를 돌려준다.
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

// runPRCreate 는 초안으로 먼저 열어 보고, 초안을 못 여는 저장소면 보통 PR 로 연다.
// 요금제에 따라 비공개 저장소는 초안을 못 쓴다. 초안 하나 때문에 PR 을 통째로
// 잃는 것보다 낫다.
func runPRCreate(path string, args []string, draft bool) ([]byte, error) {
	if draft {
		out, err := ghPR(path, append(args, "--draft"))
		if err == nil {
			return out, nil
		}
		log.Printf("[GitMgr] 초안으로 못 열었다 (%s). 보통 PR 로 연다.", firstLine(string(out)))
	}
	return ghPR(path, args)
}

func ghPR(path string, args []string) ([]byte, error) {
	cmd := exec.Command("gh", args...)
	cmd.Dir = path
	cmd.Env = os.Environ()
	return cmd.CombinedOutput()
}

// firstLine 은 첫 줄만 준다. 제목에 쓴다.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}

// prBody 는 바뀐 파일과 줄 수, 커밋 메시지를 적는다. 받는 사람이 diff 를
// 처음부터 읽지 않아도 되게 한다.
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
