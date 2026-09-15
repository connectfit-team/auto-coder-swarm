package orchestrator

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
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

// blockedByMissingContract 는 막힌 까닭이 **남의 계약** 이면 그 임자에게
// 일을 만든다. 이 저장소의 실수는 넘기지 않는다.
func (t *taskContext) blockedByMissingContract() ([]StatelessRequest, string) {
	names := missingContractNames(t.lastMissing)
	if len(names) == 0 {
		return nil, ""
	}

	contract, local := splitMissing(t.repoPath, names)
	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "MISSING_CONTRACT",
		fmt.Sprintf("없는 이름 %d개 — 남의 계약 %d, 이 저장소의 실수 %d",
			len(names), countNames(contract), len(local)),
		strings.Join(local, " · "), renderContract(contract))

	var out []StatelessRequest
	for owner, want := range contract {
		if owner == t.targetRepo || hasRepo(t.req.ParentRepos, owner) {
			continue
		}
		// **없는 저장소에 일을 만들지 않는다.** 사본이 없으면 그 작업은
		// 시작하자마자 죽고, 사람은 까닭 없는 실패 하나를 더 본다.
		if !t.orchestrator.wsMgr.HasRepo(owner) {
			t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "MISSING_CONTRACT",
				fmt.Sprintf("%s 가 이 시스템에 없다 — 사본을 받아야 한다", owner),
				"", strings.Join(want, " · "))
			continue
		}
		sort.Strings(want)
		out = append(out, StatelessRequest{
			UserRequest: fmt.Sprintf(
				"%s 에서 「%s」 를 만들려는데 계약에 이것이 없어 막혔다: %s\n"+
					"이 저장소가 그 계약의 임자다. 필드·RPC 를 더해라.",
				t.targetRepo, strings.TrimSpace(t.req.UserRequest), strings.Join(want, ", ")),
			TargetRepo:   owner,
			Depth:        t.req.Depth - 1,
			ParentRepos:  append(t.req.ParentRepos, t.targetRepo),
			ParentTaskID: rootTaskID(t),
		})
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CHAIN_TRIGGERED",
			fmt.Sprintf("계약의 임자에게 넘긴다: %s", owner), "", strings.Join(want, " · "))
	}
	if t.req.Depth <= 0 {
		out = nil
	}
	return out, missingNote(contract, local)
}

func countNames(m map[string][]string) int {
	n := 0
	for _, v := range m {
		n += len(v)
	}
	return n
}

func renderContract(m map[string][]string) string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k + ": " + strings.Join(m[k], ", ") + "\n")
	}
	return b.String()
}

// missingNote 는 사람이 읽을 실패 사유다.
//
// 계약 구멍이 하나도 없는데 "뒤쪽 저장소에 먼저 있어야 한다" 고 하면 사람은
// 엉뚱한 곳을 본다. 실제로 그렇게 적혀 나갔다(W-72062 — 남의 계약 0,
// 이 저장소의 실수 3).
func missingNote(contract map[string][]string, local []string) string {
	var b strings.Builder
	if n := countNames(contract); n > 0 {
		b.WriteString("이 저장소에 없는 이름 때문에 막혔다 — 계약의 임자에게 먼저 있어야 한다:\n")
		b.WriteString(renderContract(contract))
	}
	if len(local) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("이 저장소 안에서 없는 이름을 불렀다 — 여기서 고칠 일이다:\n  " +
			strings.Join(clipList(local), "\n  ") + "\n")
	}
	if b.Len() == 0 {
		return "없는 이름 때문에 막혔다"
	}
	return strings.TrimRight(b.String(), "\n")
}
