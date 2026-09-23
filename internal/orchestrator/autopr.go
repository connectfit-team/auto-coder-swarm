package orchestrator

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/connectfit-team/auto-coder-swarm/internal/gitmgr"
)

// 스스로 PR 까지 간다 — **켜 둔 저장소에서만.**
//
// codex·aider·hermes 는 고치고 검사하고 내놓는 것까지 한 흐름이다. 우리는
// 관문을 다 지나고도 「승인 대기」 에서 멈춰, 사람이 누르기 전에는 아무 일도
// 일어나지 않았다.
//
// 그렇다고 아무 데나 열 수는 없다. 두 가지는 그대로 둔다.
//   - 계약 저장소(protogen·proto-*)는 무슨 설정이든 밀지 않는다(gitmgr).
//   - **머지는 하지 않는다.** 초안으로 열고 사람이 본다.
//
// 기본값은 꺼 둔 것이다. SWARM_AUTO_PR_ALLOW 에 적은 저장소에서만 연다.
func autoPRAllowed(repoName string) bool {
	raw := strings.TrimSpace(os.Getenv("SWARM_AUTO_PR_ALLOW"))
	if raw == "" || repoName == "" {
		return false
	}
	for _, part := range strings.Split(raw, ",") {
		if p := strings.TrimSpace(part); p == "*" || strings.EqualFold(p, repoName) {
			return true
		}
	}
	return false
}

// openPRMyself 는 관문을 다 지난 수정을 초안 PR 로 연다.
// 열지 못하면 false — 부르는 쪽은 여느 때처럼 승인 대기로 간다.
func (t *taskContext) openPRMyself(why string) (string, bool) {
	if !autoPRAllowed(t.targetRepo) {
		return "", false
	}
	branch := t.currentBranch
	if branch == "" {
		branch = "acs-" + time.Now().Format("0102150405")
	}
	url, err := t.orchestrator.gitMgr.PushApprovedChangesOpt(
		t.repoPath, t.targetRepo, branch, prTitleFrom(t.req.UserRequest),
		gitmgr.PushOptions{
			BodyLead: autoPRLead(t.taskID, why),
			// **초안으로 연다.** 사람이 읽기 전에 머지되지 않게.
			Draft: true,
		})
	if err != nil {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "AUTO_PR_FAILED",
			"스스로 PR 을 열지 못했다 — 승인 대기로 둔다", err.Error(), "")
		return "", false
	}
	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "AUTO_PR",
		"관문을 다 지나 초안 PR 을 열었다 — 머지는 사람이 한다", why, url)
	return url, true
}

// autoPRLead 는 PR 본문 맨 앞에 붙일 글이다. 읽는 사람이 무엇인지 바로 알게 한다.
func autoPRLead(taskID, why string) string {
	return fmt.Sprintf(`> **이 PR 은 사내 자동 코딩 시스템이 스스로 만들었습니다.**
> 사람이 아직 읽지 않았습니다 — **반드시 검토한 뒤에 머지하세요.**
> 작업 %s · 넘긴 까닭: %s

`, taskID, why)
}

// prTitleFrom 은 요청문 첫 줄을 제목으로 쓴다.
func prTitleFrom(s string) string {
	line := strings.TrimSpace(firstLineOf(strings.TrimSpace(s)))
	if line == "" {
		line = "자동 코딩 시스템이 만든 수정"
	}
	return clip(line, 120)
}
