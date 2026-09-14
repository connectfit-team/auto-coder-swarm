package orchestrator

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
	"time"
)

// 이 저장소에 없는 이름 때문에 막혔으면, 그 이름이 있어야 할 저장소로 넘긴다.
//
// 「고용주웹에서 연결보류를 추가」 가 세 번 연달아 막혔는데, 늘어난 타입
// 오류가 전부 **없는 이름**이었다(W-33064).
//
//	Cannot find name 'checkIfRequestIsPending'
//	Property 'isPending' does not exist on type 'LaborContract'
//	Module '$lib/server/data/cabinet' has no exported member 'updateConnectionStatus'
//
// 모델이 게을러서가 아니다. `LaborContract.isPending` 은 proto·백엔드에서
// 오는 필드다. **연결보류는 프런트 혼자서는 못 만든다.** 그런데 그때 하는
// 말이 "최대 시도 초과" 였다 — 사람은 왜 안 됐는지 알 수 없고, 뒤쪽 저장소에
// 일이 만들어지지도 않는다.
//
// 없는 이름을 모아 눈에게 "이건 어느 저장소의 말이냐" 고 묻고, 그 저장소에
// 일을 만든다. 연쇄는 성공했을 때만 돌지만(임팩트 분석), **막혔을 때야말로
// 다른 저장소가 필요하다는 가장 분명한 신호다.**

var (
	reCannotFind  = regexp.MustCompile(`Cannot find name '([^']+)'`)
	reNoExported  = regexp.MustCompile(`has no exported member '([^']+)'`)
	rePropOnType  = regexp.MustCompile(`Property '([^']+)' does not exist on type '([^']+)'`)
	reModuleOfErr = regexp.MustCompile(`Module "?'?([^"']+)'?"? has no exported member`)
)

// missingNames 는 "없어서 못 쓴 이름" 만 모은다. 문법 오류나 타입 불일치는 뺀다.
func missingContractNames(es []typeError) []string {
	seen := map[string]bool{}
	for _, e := range es {
		for _, m := range reCannotFind.FindAllStringSubmatch(e.msg, -1) {
			seen[m[1]] = true
		}
		for _, m := range reNoExported.FindAllStringSubmatch(e.msg, -1) {
			seen[m[1]] = true
		}
		// 속성은 그것을 담은 타입까지 적어야 어느 저장소의 말인지 알 수 있다.
		for _, m := range rePropOnType.FindAllStringSubmatch(e.msg, -1) {
			seen[m[2]+"."+m[1]] = true
		}
	}
	var out []string
	for n := range seen {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// blockedByMissingContract 는 막힌 까닭이 "없는 이름" 이면 그 이름의 임자
// 저장소에 일을 만든다. 아니면 빈 목록이다.
func (t *taskContext) blockedByMissingContract() []StatelessRequest {
	names := missingContractNames(t.lastMissing)
	if len(names) == 0 || t.req.Depth <= 0 {
		return nil
	}

	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "MISSING_CONTRACT",
		fmt.Sprintf("이 저장소에 없는 이름 %d개 때문에 막혔다", len(names)),
		"", strings.Join(names, " · "))

	// 이름 그대로 물어본다. 눈은 코드 색인에서 그 이름이 실제로 어디 있는지,
	// 없으면 어느 저장소의 말인지를 준다.
	q := fmt.Sprintf("%s 에 %s 이(가) 필요하다. %s",
		t.targetRepo, strings.Join(names, ", "), strings.TrimSpace(t.req.UserRequest))

	sub, cancel := context.WithTimeout(t.ctx, 30*time.Second)
	defer cancel()
	routed, err := t.orchestrator.insightClient.RouteRepos(sub, q)
	if err != nil {
		log.Printf("[Orchestrator] 없는 이름의 임자를 못 찾았다: %v", err)
		return nil
	}

	for _, r := range routed {
		if r.RepoName == t.targetRepo || hasRepo(t.req.ParentRepos, r.RepoName) {
			continue
		}
		req := StatelessRequest{
			UserRequest: fmt.Sprintf(
				"%s 에서 「%s」 를 만들려는데 이 이름들이 없어 막혔다: %s\n"+
					"이 저장소에 먼저 있어야 한다. 계약(필드·RPC·함수)을 더해라.",
				t.targetRepo, strings.TrimSpace(t.req.UserRequest), strings.Join(names, ", ")),
			TargetRepo:   r.RepoName,
			Depth:        t.req.Depth - 1,
			ParentRepos:  append(t.req.ParentRepos, t.targetRepo),
			ParentTaskID: rootTaskID(t),
		}
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CHAIN_TRIGGERED",
			fmt.Sprintf("없는 이름의 임자에게 넘긴다: %s", r.RepoName), "", strings.Join(names, " · "))
		return []StatelessRequest{req}
	}
	return nil
}

// missingContractNote 는 사람이 읽을 실패 사유다.
func missingContractNote(names []string) string {
	return "이 저장소에 없는 이름 때문에 막혔다 — 뒤쪽 저장소에 먼저 있어야 한다:\n  " +
		strings.Join(names, "\n  ")
}
