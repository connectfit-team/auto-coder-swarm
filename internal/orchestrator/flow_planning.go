package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
)

func (t *taskContext) stepPlanning(attempt int) error {
	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "PLANNING", fmt.Sprintf("계획 수립 (시도 %d/3)", attempt), t.analysis, "")

	// Inject Analysis AND CKH Knowledge into Planning
	input := t.analysis
	if t.ckhKnowledge != "" {
		input = fmt.Sprintf("[CORPORATE KNOWLEDGE]\n%s\n\n[CODE ANALYSIS]\n%s", t.ckhKnowledge, input)
	}

	if len(t.actionablePath) > 0 {
		input = fmt.Sprintf("[고칠 파일 — 이 중에서 골라라]\n%s\n\n%s",
			strings.Join(t.actionablePath, "\n"), input)
	}

	if b := skillDigest(t.skills); b != "" {
		input = b + "\n" + input
	}

	if t.lastFeedback != "" {
		input += "\n\nFEEDBACK:\n" + t.lastFeedback
	}

	// **분석이 이미 짚었으면 다시 묻지 않는다.**
	//
	// 고장 질문에는 CIE 가 파일과 문제의 줄을 짚어 준다. 그걸 다시 모델에게
	// 넘겨 "어느 파일을 고칠까" 를 물으면, 그 되물음에서 엉뚱한 화면 파일로
	// 새는 일이 잦다. 아는 것은 그대로 쓴다.
	if direct := agent.PlanFromDefectReport(t.analysis); len(direct) > 0 && t.lastFeedback == "" {
		plan := agent.Plan{RepoName: t.req.TargetRepo, Changes: direct}
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "PLAN_FROM_ANALYSIS",
			fmt.Sprintf("분석이 짚은 파일 %d개를 그대로 계획으로 씁니다", len(direct)),
			"", planSummary(direct))
		return t.finishPlanning(plan, attempt)
	}

	voteRes, _ := t.voter.Vote(t.ctx, "Planner", t.planner.BuildPrompt(input))

	// **이긴 답이 JSON 이 아니면 나머지 후보를 써 본다.**
	//
	// 투표는 같은 답이 몇 번 나왔는지로만 이긴다 — 그 답이 읽히는지는 보지
	// 않는다. 실제로 이긴 답에 따옴표가 하나 잘못 들어가(`"점검 및"));`)
	// 파싱이 깨졌고, 멀쩡한 다른 후보가 있는데도 작업이 통째로 실패했다.
	plan, err := t.planner.ParsePlanWithRepo(voteRes.Winner, t.req.TargetRepo)
	if err != nil {
		firstErr := err
		for _, alt := range voteRes.Details {
			if alt == "" || alt == voteRes.Winner {
				continue
			}
			if p2, e2 := t.planner.ParsePlanWithRepo(alt, t.req.TargetRepo); e2 == nil {
				t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "PLAN_ALT_USED",
					"이긴 답이 JSON 으로 안 읽혀 다른 후보를 썼습니다", "", firstErr.Error())
				plan, err = p2, nil
				break
			}
		}
		if err != nil {
			return fmt.Errorf("어느 후보도 계획으로 읽히지 않는다: %w", firstErr)
		}
	}

	return t.finishPlanning(plan, attempt)
}

// finishPlanning 은 계획이 정해진 뒤의 검사와 준비를 한다.
//
// 계획이 어디서 왔든(모델이 세웠든, 분석에서 그대로 왔든) 거쳐야 하는 길이
// 같아서 한 곳에 모았다.
func (t *taskContext) finishPlanning(plan agent.Plan, attempt int) error {
	// **변경이 하나도 없는 계획은 계획이 아니다.**
	//
	// 빈 계획이 오면 아무 파일도 안 쓰고, 빈 diff 가 만들어지고, 검토 두 관문이
	// 그걸 통과시켜 **아무것도 안 한 작업이 "성공" 으로 기록됐다.**
	if len(plan.Changes) == 0 {
		if attempt < 3 {
			t.lastFeedback = "PLAN REJECTED: changes 가 비어 있다. 고칠 파일을 최소 하나 고르고, " +
				"그 파일에서 무엇을 어떻게 바꿀지 적어라.\n" + t.candidateHint
			return errRetryPlanning
		}
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

	// **이 저장소에 없는 파일은 계획에서 뺀다.**
	//
	// cms 작업 계획에 statistics/internal/event-synchronizer/workplace.go 가
	// 들어왔다 — 다른 저장소 파일이다. 그대로 두면 코더가 그 자리에 새 파일을
	// 만들어 저장소에 없던 코드를 심는다. 폴더까지 없으면 지어낸 것으로 본다.
	// 폴더가 있으면 새 파일을 만들려는 것일 수 있으므로 살린다.
	if t.repoPath != "" {
		kept, dropped := splitByRepoReality(t.repoPath, plan.Changes)
		if len(dropped) > 0 {
			t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "PLAN_TRIMMED",
				fmt.Sprintf("%s 에 없는 경로 %d개를 계획에서 뺐습니다", t.targetRepo, len(dropped)),
				"", strings.Join(dropped, "\n"))
		}
		if len(kept) == 0 {
			// **되살릴 수 있는 실패는 되먹임을 주고 다시 계획하게 한다.**
			//
			// 계획이 프롬프트 예시 path/to/file.ext 를 그대로 베껴 오기도 한다.
			// 여기서 죽이면 남은 두 번의 기회를 못 쓴다. 무엇이 왜 틀렸는지와
			// **실제로 있는 파일 이름**을 함께 돌려준다.
			t.lastFeedback = fmt.Sprintf(
				"PLAN REJECTED: 다음 경로는 %s 에 없다 — %s\n"+
					"file_path 는 아래 실제 파일 중에서 고르고, 예시 경로(path/to/file.ext)를 그대로 쓰지 마라.\n%s",
				t.targetRepo, strings.Join(dropped, ", "), t.candidateHint)
			return errRetryPlanning
		}
		plan.Changes = kept
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

// splitByRepoReality 는 계획의 변경을 실재하는 것과 지어낸 것으로 가른다.
//
// 파일이 이미 있으면 그대로 둔다. 없더라도 담길 **폴더가 있으면** 새로 만들려는
// 것으로 보고 살린다. 폴더까지 없으면 이 저장소 이야기가 아니다.
func splitByRepoReality(repoPath string, changes []agent.FileChange) (kept []agent.FileChange, dropped []string) {
	for _, c := range changes {
		rel := strings.TrimSpace(c.FilePath)
		// 저장소 밖으로 나가는 경로는 무조건 뺀다.
		if rel == "" || strings.HasPrefix(rel, "/") || strings.Contains(rel, "..") {
			dropped = append(dropped, c.FilePath)
			continue
		}
		full := filepath.Join(repoPath, rel)
		if _, err := os.Stat(full); err == nil {
			kept = append(kept, c)
			continue
		}
		if fi, err := os.Stat(filepath.Dir(full)); err == nil && fi.IsDir() {
			kept = append(kept, c)
			continue
		}
		dropped = append(dropped, c.FilePath)
	}
	return kept, dropped
}

// planSummary 는 계획을 한눈에 보이게 적는다.
func planSummary(changes []agent.FileChange) string {
	var b strings.Builder
	for _, c := range changes {
		b.WriteString("- " + c.FilePath + "\n")
	}
	return b.String()
}
