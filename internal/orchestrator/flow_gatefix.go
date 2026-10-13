package orchestrator

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
)

// 관문 위반을 코딩 단계 안에서 고쳐 보는 횟수.
const gateFixRounds = 2

// stepGateFix 는 넘기기 전 관문을 **코딩 단계 안에서** 미리 돌린다.
//
// 관문은 본래 빌드·비평·검토를 다 지난 맨 끝에서 운다. 위반 하나에 시도 하나를
// 통째로 쓰는데 시도는 셋뿐이라, 실측에서 떨어진 넷이 전부 「관문 위반을 세 시도
// 안에 못 고친 것」 이었다. 여기서 바로 들려주면 같은 시도 안에서 고친다.
//
// 되돌려야 하는 위반(엉뚱한 것을 고쳤다)은 건드리지 않는다 — 그건 계획부터 다시다.
func (t *taskContext) stepGateFix() {
	// **먼저 도구를 쥐여 준다.** 안 되면 기계식으로 물러선다.
	if t.agenticGateFix() {
		return
	}
	plan, ok := t.ctx.Value("current_plan").(agent.Plan)
	if !ok {
		return
	}
	targets := protoTargets(plan)
	if len(targets) == 0 {
		return
	}

	for round := 1; round <= gateFixRounds; round++ {
		g, ok := firstFixable(t.gateChecks(t.currentDiff()))
		if !ok {
			return
		}
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "GATE_FIX",
			fmt.Sprintf("관문 위반을 코딩 단계에서 고친다 (%d/%d) — %s", round, gateFixRounds, g.label),
			g.feedback, g.note)

		for _, rel := range targets {
			instr := g.feedback +
				"\n\n이 파일에서 **위에 적힌 자리만** 고쳐라. 나머지는 그대로 두어라."
			if _, err := t.coder.ModifyFile(t.ctx, filepath.Join(t.repoPath, rel), instr); err != nil {
				t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "GATE_FIX_FAILED",
					fmt.Sprintf("[%s] 관문 위반을 못 고쳤다", rel), "", err.Error())
			}
		}
	}
}

// firstFixable 은 여기서 고쳐 볼 수 있는 첫 위반이다.
// 되돌려야 하는 것은 고르지 않는다.
func firstFixable(bad []gateResult) (gateResult, bool) {
	for _, g := range bad {
		if !g.revert {
			return g, true
		}
	}
	return gateResult{}, false
}

// protoTargets 는 계획이 짚은 계약 파일이다.
func protoTargets(plan agent.Plan) []string {
	var out []string
	for _, c := range plan.Changes {
		if strings.HasSuffix(c.FilePath, ".proto") {
			out = append(out, c.FilePath)
		}
	}
	return out
}
