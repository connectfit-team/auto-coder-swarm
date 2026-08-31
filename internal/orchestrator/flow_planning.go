package orchestrator

import (
	"context"
	"fmt"
	"path/filepath"
	"time"
)

func (t *taskContext) stepPlanning(attempt int) error {
	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "PLANNING", fmt.Sprintf("계획 수립 (시도 %d/3)", attempt), t.analysis, "")

	// Inject Analysis AND CKH Knowledge into Planning
	input := t.analysis
	if t.ckhKnowledge != "" {
		input = fmt.Sprintf("[CORPORATE KNOWLEDGE]\n%s\n\n[CODE ANALYSIS]\n%s", t.ckhKnowledge, input)
	}

	if b := skillDigest(t.skills); b != "" {
		input = b + "\n" + input
	}

	if t.lastFeedback != "" {
		input += "\n\nFEEDBACK:\n" + t.lastFeedback
	}

	voteRes, _ := t.voter.Vote(t.ctx, "Planner", t.planner.BuildPrompt(input))
	plan, err := t.planner.ParsePlanWithRepo(voteRes.Winner, t.req.TargetRepo)
	if err != nil {
		return err
	}

	// **변경이 하나도 없는 계획은 계획이 아니다.**
	//
	// 빈 계획이 오면 아무 파일도 안 쓰고, 빈 diff 가 만들어지고, 검토 두 관문이
	// 그걸 통과시켜 **아무것도 안 한 작업이 "성공" 으로 기록됐다.**
	if len(plan.Changes) == 0 {
		return fmt.Errorf("계획에 고칠 파일이 하나도 없다")
	}

	if attempt == 1 {
		t.targetRepo = plan.RepoName
		if t.req.TargetRepo != "" {
			t.targetRepo = t.req.TargetRepo
		}
		if t.repoLockFunc != nil {
			t.repoLockFunc(t.targetRepo)
		}

		t.repoPath = filepath.Join(t.wsPath, "repo")
		t.currentBranch = fmt.Sprintf("acs-fix-%s", time.Now().Format("0102150405"))
		t.orchestrator.wsMgr.CreateWorktree(t.targetRepo, t.repoPath, t.currentBranch)
		t.meta = t.orchestrator.detectProjectTypeLLM(t.ctx, t.taskID, t.repoPath)

		// **표식 파일이 있으면 그쪽을 믿는다.**
		//
		// 모델은 빈 명령을 내기도 하고(`bash -c ""` 는 exit 0 이라 아무것도
		// 검증하지 않고 "성공" 으로 지나간다), 없는 경로를 지어내기도 한다
		// (blog-api 에 없는 `go build ./main.go`). go.mod 가 있으면 Go 라는 건
		// 모델의 판단이 아니라 사실이다.
		if kind, build := detectProjectFallback(t.repoPath); build != "" {
			if t.meta.BuildCommand != build {
				t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "BUILD_COMMAND",
					fmt.Sprintf("파일로 정함: %s (%s) — 모델 제안: %q", build, kind, t.meta.BuildCommand), "", "")
			}
			t.meta.Type, t.meta.BuildCommand = kind, build
		} else if t.meta.BuildCommand == "" {
			return fmt.Errorf("빌드 명령을 정할 수 없다 — 표식 파일(go.mod 등)도 모델 제안도 없다")
		}

		// **처음부터 안 되는 빌드를 모델 탓으로 보고하지 않는다.**
		//
		// 새 worktree 에는 의존성도 생성물도 없어서, 무슨 코드를 쓰든 검증이
		// 실패한다(실측: `vite build` → `sh: 1: vite: not found`). 그대로 두면
		// 자가치유 세 번을 태우고 "빌드 실패" 로 끝나, 원인이 코드인지 환경인지
		// 구별되지 않는다. 손대기 전에 한 번 돌려 본다.
		if t.meta.BuildCommand != "" {
			if out, err := shellCmd(t.ctx, t.repoPath, t.meta.BuildCommand).CombinedOutput(); err != nil {
				t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "BASELINE_BUILD",
					"손대기 전부터 빌드가 안 된다", t.meta.BuildCommand, string(out))
				return fmt.Errorf("이 저장소는 작업공간에서 빌드되지 않는다 — 코드가 아니라 환경 문제다: %s", t.meta.BuildCommand)
			}
			t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "BASELINE_BUILD", "기준 빌드 통과", t.meta.BuildCommand, "")
		}

		if t.meta.BenchCommand != "" {
			cmd := shellCmd(t.ctx, t.repoPath, t.meta.BenchCommand)
			bOut, _ := cmd.CombinedOutput()
			t.preBench = string(bOut)
		}
	}

	// 계획이 나와야 **실제로 건드릴 파일**의 확장자를 안다. 요청문에 "go" 라고
	// 안 적혀 있어도 Go 규약이 붙어야 하므로 여기서 다시 받는다.
	var paths []string
	for _, c := range plan.Changes {
		paths = append(paths, c.FilePath)
	}
	if exts := planExtensions(paths); len(exts) > 0 {
		t.fetchSkills(t.targetRepo, exts)
	}
	t.coder.SetConventions(skillDigest(t.skills))

	t.orchestrator.store.AddLog(t.taskID, "PLAN", fmt.Sprintf("파일 %d개 수정 계획 수립", len(plan.Changes)))
	t.ctx = context.WithValue(t.ctx, "current_plan", plan)
	return nil
}
