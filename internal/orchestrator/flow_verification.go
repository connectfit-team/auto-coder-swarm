package orchestrator

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
	"github.com/connectfit-team/auto-coder-swarm/internal/healing"
)

func (t *taskContext) stepVerification() (bool, error) {
	for healAttempt := 1; healAttempt <= 3; healAttempt++ {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "BUILD", fmt.Sprintf("[%s] 검증 (%s) - 시도 %d", t.meta.Type, t.meta.BuildCommand, healAttempt), t.meta.BuildCommand, "")
		bCmd := shellCmd(t.ctx, t.repoPath, t.meta.BuildCommand)
		buildOut, err := bCmd.CombinedOutput()

		// 빌드가 통과했으면 **바뀐 패키지의 테스트를 실제로 돌린다.**
		// 컴파일만 보면 틀린 테스트가 통과한다 — 테스트를 쓰는 것이 일의
		// 절반인데 그게 도는지 확인하지 않으면 절반은 검증되지 않은 셈이다.
		if err == nil {
			if out, ok := t.runChangedTests(t.ctx.Value("current_plan").(agent.Plan)); !ok {
				t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "TEST_FAIL", "바뀐 패키지의 테스트가 실패했다", "", clip(out, 1500))
				buildOut = []byte(out)
				err = errChangedTestsFailed
			}
		}

		if err == nil {
			// Build succeeded
			if t.meta.BenchCommand != "" {
				cmd := shellCmd(t.ctx, t.repoPath, t.meta.BenchCommand)
				bOut, _ := cmd.CombinedOutput()
				t.postBench = string(bOut)
			}
			return true, nil
		}

		if t.ctx.Err() != nil {
			return false, t.ctx.Err()
		}

		// **경고를 걷어내고 오류만 넘긴다.**
		//
		// vite 는 손대지도 않은 파일의 A11y 경고를 앞에 잔뜩 찍는다. 그 앞부분이
		// 그대로 치유기에 들어가서, 8,192 토큰이 경고로 차고 진짜 오류는 잘려
		// 나갔다. 기준 빌드는 같은 경고를 달고도 통과했으니 그건 원인이 아니다.
		failure := distillBuildError(string(buildOut))
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "HEALING_DIAGNOSIS", fmt.Sprintf("빌드 실패, 자가 치유 가동 (%d/3)", healAttempt), failure, "")

		relevantFiles := make(map[string]string)
		plan := t.ctx.Value("current_plan").(agent.Plan)
		for _, change := range plan.Changes {
			content, _ := os.ReadFile(filepath.Join(t.repoPath, change.FilePath))
			relevantFiles[change.FilePath] = string(content)
		}

		healingPlan, hErr := t.healer.ProposeHealing(t.ctx, failure, t.meta.Type, relevantFiles)
		if hErr != nil {
			t.lastFeedback = fmt.Sprintf("HEALER LLM CRASHED: %v\nBUILD ERROR:\n%s", hErr, failure)
			exec.CommandContext(t.ctx, "git", "-C", t.repoPath, "checkout", ".").Run()
			return false, nil
		}

		abort := false
		for _, step := range healingPlan.Steps {
			switch step.Action {
			case healing.ActionModifyCode:
				// **치유는 계획한 파일과 오류가 지목한 파일까지만 건드린다.**
				//
				// 말일 경계 하나를 고치라고 했는데 치유기가 export API 를
				// 함께 뜯어고쳐, 요청하지 않은 workAt 필터가 diff 에 섞여
				// 승인 대기까지 갔다. 빌드가 지목하지 않은 파일이라면 이
				// 작업과 상관이 없다.
				if !allowedToHeal(step.TargetFile, plan, failure) {
					t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "HEALING_BLOCKED",
						fmt.Sprintf("[%s] 계획에도 오류에도 없는 파일이라 건드리지 않습니다", step.TargetFile), "", "")
					continue
				}
				if _, err := t.coder.ModifyFile(t.ctx, filepath.Join(t.repoPath, step.TargetFile), step.Instruction); err != nil {
					t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "HEALING_FAILED",
						fmt.Sprintf("[%s] 치유가 파일을 못 고쳤습니다", step.TargetFile), "", err.Error())
				}
			case healing.ActionRunCommand:
				shellCmd(t.ctx, t.repoPath, step.Command).Run()
			case healing.ActionAbort:
				abort = true
			}
		}

		if abort {
			t.lastFeedback = fmt.Sprintf("HEALER ABORTED: %s\nBUILD ERROR:\n%s", healingPlan.Diagnosis, string(buildOut))
			exec.CommandContext(t.ctx, "git", "-C", t.repoPath, "checkout", ".").Run()
			return false, nil
		}
	}

	// Exhausted all heal attempts
	t.lastFeedback = "HEALER FAILED TO FIX BUILD AFTER 3 ATTEMPTS"
	exec.CommandContext(t.ctx, "git", "-C", t.repoPath, "checkout", ".").Run()
	return false, nil
}

// findWipedFiles 는 거의 통째로 지워진 파일을 찾는다.
//
// git diff --numstat 이 파일마다 "더한 줄\t지운 줄\t경로" 를 준다. 파일을
// 통째로 지우는 것이 목적인 작업도 있으므로, 지운 양이 크고 더한 양이
// 하찮을 때만 막는다. 이진 파일은 숫자 대신 "-" 가 와서 저절로 걸러진다.
func findWipedFiles(ctx context.Context, repoPath string) []string {
	out, err := exec.CommandContext(ctx, "git", "-C", repoPath, "diff", "--numstat", "HEAD").Output()
	if err != nil {
		return nil
	}
	var wiped []string
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Split(strings.TrimSpace(line), "\t")
		if len(f) < 3 {
			continue
		}
		added, err1 := strconv.Atoi(f[0])
		deleted, err2 := strconv.Atoi(f[1])
		if err1 != nil || err2 != nil {
			continue
		}
		if deleted >= wipeMinDeleted && deleted >= added*wipeRatio {
			wiped = append(wiped, fmt.Sprintf("%s (+%d/-%d)", f[2], added, deleted))
		}
	}
	return wiped
}

// allowedToHeal 은 치유가 그 파일을 건드려도 되는지 본다.
//
// 계획한 파일이거나, 빌드 오류가 이름을 댄 파일이어야 한다. 그 밖의 파일을
// 고치는 것은 이 작업이 하기로 한 일이 아니다.
func allowedToHeal(target string, plan agent.Plan, buildError string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	for _, c := range plan.Changes {
		if sameFilePath(c.FilePath, target) {
			return true
		}
	}
	// 오류 메시지에 **전체 경로**가 그대로 찍힌다:
	// `"getX" is not exported by "src/lib/server/db/attendance.ts"`
	//
	// 파일 이름만으로 맞추면 안 된다 — +server.ts·index.ts·main.go 처럼
	// 같은 이름이 저장소에 수십 개다. 실제로 오류에 찍힌 다른 +server.ts
	// 때문에 상관없는 파일이 통과했다.
	return strings.Contains(buildError, target)
}

// revertUnplannedFiles 는 계획에 없는데 바뀐 파일을 되돌린다.
func (t *taskContext) revertUnplannedFiles(plan agent.Plan) []string {
	out, err := exec.CommandContext(t.ctx, "git", "-C", t.repoPath, "diff", "--name-only", "HEAD").Output()
	if err != nil {
		return nil
	}
	var reverted []string
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.TrimSpace(line)
		if f == "" {
			continue
		}
		planned := false
		for _, c := range plan.Changes {
			if sameFilePath(c.FilePath, f) {
				planned = true
				break
			}
		}
		if planned {
			continue
		}
		exec.CommandContext(t.ctx, "git", "-C", t.repoPath, "checkout", "--", f).Run()
		reverted = append(reverted, f)
	}
	return reverted
}

// markAnalysisDeadEnd 는 이번 계획의 파일을 막다른 길로 표시한다.
// 분석에서 나온 계획일 때만 뜻이 있다 — 모델이 세운 계획은 다음 시도에
// 어차피 달라진다.
func (t *taskContext) markAnalysisDeadEnd() []string {
	if !t.planFromAnalysis {
		return nil
	}
	plan, ok := t.ctx.Value("current_plan").(agent.Plan)
	if !ok {
		return nil
	}
	var added []string
	for _, c := range plan.Changes {
		if c.FilePath == "" {
			continue
		}
		t.deadPaths = append(t.deadPaths, c.FilePath)
		added = append(added, c.FilePath)
	}
	if len(added) > 0 {
		t.lastFeedback = "앞 시도에서 " + strings.Join(added, ", ") +
			" 를 고치려 했으나 고칠 것이 없었다. 그 파일은 원인이 아니다."
	}
	return added
}
