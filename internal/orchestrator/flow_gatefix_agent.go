package orchestrator

import (
	"fmt"
	"os"
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/agentloop"
)

// 도구를 쥔 고치기가 쓸 수 있는 턴. 창이 16,384 라 길게 못 간다.
const gateFixTurns = 6

// agenticGateFix 는 모델이 **스스로 관문을 보며** 고치게 한다.
//
// 기계식(stepGateFix)은 위반 하나를 골라 계약 파일 전부에 같은 되먹임을 들려
// 준다. 어느 파일이 문제인지, 무엇을 더 봐야 하는지는 모델이 더 잘 안다 —
// check 로 직접 보고, read_file·search_code 로 확인하고, edit_file 로 고친다.
//
// 편집 기계는 그대로다. 루프는 「무엇을 할지」 를 정하고 「어떻게 고칠지」 는
// 코더가 한다(editViaCoder).
//
// 끄려면 SWARM_GATEFIX_AGENT=0.
func (t *taskContext) agenticGateFix() bool {
	if strings.TrimSpace(os.Getenv("SWARM_GATEFIX_AGENT")) == "0" {
		return false
	}
	if t.primaryLLM == nil || t.repoPath == "" {
		return false
	}
	g, ok := firstFixable(t.gateChecks(t.currentDiff()))
	if !ok {
		return true // 고칠 것이 없다
	}

	tools, dispatch := t.coderTools()
	loop := &agentloop.Loop{
		LLM:      t.primaryLLM,
		Tools:    tools,
		Call:     dispatch,
		MaxTurns: gateFixTurns,
		Log: func(kind, msg string) {
			t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "GATE_FIX_"+kind, msg, "", "")
		},
	}

	system := "너는 계약(.proto)을 고치는 코더다.\n" +
		"지금 고친 것이 관문에 걸렸다. **걸린 자리만** 고쳐라 — 처음부터 다시 쓰지 마라.\n" +
		"지어내지 마라. 이름이 이미 있는지는 search_code 로, 파일 내용은 read_file 로 확인해라.\n" +
		"고친 뒤에는 **반드시 check 를 불러** 관문을 지나는지 보라.\n" +
		"check 가 「관문을 지난다」 고 하면 더 고치지 말고 끝내라."

	goal := "아래 관문에 걸렸다. 고쳐라.\n\n## " + g.label + "\n" + g.feedback

	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "GATE_FIX_AGENT",
		fmt.Sprintf("도구를 쥐고 관문 위반을 고친다 — %s", g.label), g.feedback, g.note)

	if _, err := loop.Run(t.ctx, system, goal); err != nil {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "GATE_FIX_AGENT_FAILED",
			"도구로 고치지 못했다 — 기계식으로 물러선다", "", err.Error())
		return false
	}

	// 정말 지나는지는 **우리가** 본다. 모델이 그렇다고 해서 믿지 않는다.
	if _, still := firstFixable(t.gateChecks(t.currentDiff())); still {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "GATE_FIX_AGENT_SHORT",
			"도구로 고쳤지만 아직 걸린다 — 기계식으로 더 해 본다", "", "")
		return false
	}
	return true
}
