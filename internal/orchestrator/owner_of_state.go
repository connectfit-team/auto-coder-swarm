package orchestrator

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
)

// 담을 자리가 없을 때 **어느 계약에 붙어야 하는지**는 이미 알 수 있다.
//
// 「연결 요청에 보류 상태를 담을 필드」 가 없다고 했으면, 그 「연결 요청」 이
// 어떤 타입인지 물어보면 된다. 그 타입이 생성물 안에 있으면 경로가 임자를
// 말해 준다 — `…/protos/ceowebapis/…` → `proto-ceowebapis`(#76 과 같은 길).
//
// 산문으로 저장소를 고르게 하면 빗나간다. 이름 하나를 묻고 경로를 읽는다.

// askTypeForState 는 그 상태가 붙어야 할 **타입 이름 하나**를 묻는다.
func (t *taskContext) askTypeForState(files []string, missing []string) string {
	sheet := t.stateSheet(files)
	if strings.TrimSpace(sheet) == "" {
		return ""
	}
	prompt := fmt.Sprintf(`아래는 이 일과 맞닿은 자리에 **실제로 있는 이름과 타입의 필드**다.

%s
[없어서 못 만드는 것]
%s

이 상태는 **어느 타입에 붙어야 하나?** 위 목록에 있는 타입 이름 하나만
적어라. 다른 말은 쓰지 마라.`, sheet, strings.Join(missing, "\n"))

	ctx, cancel := context.WithTimeout(t.ctx, stateCheckTimeout)
	defer cancel()
	raw, err := agent.CallLLM(ctx, t.primaryLLM, "StateOwner", prompt)
	if err != nil {
		return ""
	}
	name := strings.TrimSpace(strings.SplitN(strings.TrimSpace(raw), "\n", 2)[0])
	name = strings.Trim(name, "`'\"., ")
	if i := strings.LastIndex(name, "."); i > 0 {
		name = name[:i] // Type.field 로 답했으면 타입만
	}
	if name == "" || strings.ContainsAny(name, " \t") {
		return ""
	}
	return name
}

// ownerRepoForState 는 그 상태가 붙을 타입의 임자 저장소를 준다.
// 못 찾으면 빈 문자열과 까닭을 준다.
func (t *taskContext) ownerRepoForState(files, missing []string) (string, string) {
	// 먼저 기계가 따라가 본다. 그 파일이 다루는 데이터가 어느 계약에서
	// 오는지는 적혀 있는 사실이다.
	traced, seen := traceContracts(files, func(f string) (string, string, string) {
		return protoOwnerViaClientDeep(t.repoPath, f, 2)
	})

	// 하나로 확정됐으면 묻지 않는다 — 고를 것이 없는 자리다.
	if len(traced) == 1 {
		return t.resolveTracedOwner(seen[traced[0]])
	}

	// 고르는 자리에는 저장소가 쓰는 계약을 다 보인다. 계획이 짚은 파일로
	// 미리 좁히면 계획이 빗나간 회차에 후보가 통째로 사라지거나 엉뚱한
	// 것만 남는다.
	scan := repoContracts(t.repoPath)
	menu := scan.contracts
	for k, c := range seen {
		if m, ok := menu[k]; ok {
			m.why = c.why // 표가 달릴 줄에는 실제로 따라간 근거를 남긴다
			menu[k] = m
			continue
		}
		menu[k] = c
	}
	if note := scan.note(); note != "" {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CONTRACT_SCAN",
			note, fmt.Sprintf("훑은 파일 %d개 · 공장 정의 %d개 · 계약 %d개",
				scan.scanned, scan.factories, len(scan.contracts)), "")
	}
	// 잘린 목록을 온전한 것처럼 내놓고 「어느 것도 아니면 0」 을 물으면 안 된다.
	if len(menu) > 0 && !scan.truncated {
		order := make([]string, 0, len(menu))
		for k := range menu {
			order = append(order, k)
		}
		sort.Strings(order)
		ev := map[string]string{}
		for _, k := range order {
			line := fmt.Sprintf("%s (%s)", menu[k].why, menu[k].owner)
			if _, hit := seen[k]; hit {
				line += "  ← 고칠 파일이 실제로 쓰는 계약"
			}
			if menu[k].note != "" {
				line += "\n     그 계약에 적힌 말: " + menu[k].note
			}
			ev[k] = line
		}
		picked, why := t.pickContractAmong(order, ev, missing)
		if picked != "" {
			if _, hit := seen[picked]; !hit && len(traced) > 0 {
				// 기계가 따라간 것과 다른 것을 골랐다. 쓰되 남긴다.
				t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CONTRACT_PICK_DIFFERS",
					"고른 계약이 고칠 파일이 쓰는 것과 다르다", strings.Join(traced, ", "), picked)
			}
			c := menu[picked]
			c.why = fmt.Sprintf("%s · %s", c.why, why)
			return t.resolveTracedOwner(c)
		}
		if len(traced) > 1 {
			return "", why
		}
	}

	typeName := t.askTypeForState(files, missing)
	if typeName == "" {
		return "", "어느 타입에 붙어야 하는지 답을 못 받았다"
	}
	home := typeHome(t.repoPath, typeName)
	if home == "" {
		return "", fmt.Sprintf("%s 가 이 저장소에 정의돼 있지 않다", typeName)
	}
	owner := protoOwnerRepo(home)
	if owner == "" {
		// **손으로 쓴 타입이어도 멈출 자리가 아니다.**
		//
		// 그 타입을 채우는 것은 RPC 다. 그 파일이 부르는 공장을 따라가면
		// 계약의 임자가 나온다 — 한 걸음도 모델에게 묻지 않는다(W-77123).
		if o, _, why := protoOwnerViaClientDeep(t.repoPath, home, 2); o != "" {
			if !t.orchestrator.wsMgr.HasRepo(o) {
				return "", fmt.Sprintf("%s 가 이 시스템에 없다 — 사본을 받아야 한다 (%s)", o, why)
			}
			return o, fmt.Sprintf("%s 는 %s 가 쓴 타입이고, 그 데이터는 %s", typeName, home, why)
		} else if why != "" {
			return "", fmt.Sprintf("%s(%s) 에서 계약을 따라가지 못했다 — %s", typeName, home, why)
		}
		return "", fmt.Sprintf("%s 는 이 저장소가 손으로 쓴 타입이다(%s) — 여기서 고칠 일이다", typeName, home)
	}
	if !t.orchestrator.wsMgr.HasRepo(owner) {
		return "", fmt.Sprintf("%s 가 이 시스템에 없다 — 사본을 받아야 한다", owner)
	}
	return owner, fmt.Sprintf("%s 는 %s 에서 온다", typeName, home)
}

// resolveTracedOwner 는 따라가서 찾은 계약을 실제로 넘길 저장소로 바꾼다.
//
// 펴낸 결과물이 아니라 원본을 고쳐야 하므로 생성물 경로로 원본 저장소와
// 발행 목표를 찾는다. 경로는 값으로 들고 다닌다 — 까닭 글에서 다시
// 긁어내면 고칠 파일 경로가 계약 경로 자리에 들어간다.
func (t *taskContext) resolveTracedOwner(c tracedContract) (string, string) {
	if c.contract != "" {
		if src, rel, target := t.protoSource(c.contract); src != "" {
			t.protoPath, t.protoTarget = rel, target
			return src, fmt.Sprintf("%s · 원본은 %s 의 %s 다 (펴내기: %s)", c.why, src, rel, target)
		}
	}
	if !t.orchestrator.wsMgr.HasRepo(c.owner) {
		return "", fmt.Sprintf("%s 가 이 시스템에 없다 — 사본을 받아야 한다 (%s)", c.owner, c.why)
	}
	return c.owner, c.why
}
