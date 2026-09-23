package orchestrator

import (
	"context"
	"fmt"
	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
)

// candidateFiles 는 이 일과 맞닿은 파일들이다. 분석이 짚은 것을 쓴다.
func (t *taskContext) candidateFiles() []string {
	var out []string
	for _, p := range pathsInText(t.analysis) {
		out = append(out, p)
		if len(out) >= 6 {
			break
		}
	}
	return out
}

// chainForMissingState 는 담을 자리가 없을 때 그 계약의 임자에게 일을 만든다.
//
// 임자를 모르면 만들지 않는다 — 없는 저장소에 일을 만들면 시작하자마자 죽고
// 사람은 까닭 없는 실패를 하나 더 볼 뿐이다.
func (t *taskContext) chainForMissingState(missing []string) []StatelessRequest {
	if len(missing) == 0 {
		return nil
	}
	if t.req.Depth <= 0 {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CHAIN_SKIPPED",
			"연쇄 깊이가 남지 않아 다른 저장소에 일을 만들지 않는다", "", "")
		return nil
	}

	// **타입에서 임자를 찾는다.** 산문으로 저장소를 고르게 하면 빗나간다.
	if plan, ok := t.ctx.Value("current_plan").(agent.Plan); ok {
		var files []string
		for _, c := range plan.Changes {
			files = append(files, c.FilePath)
		}
		if owner, why := t.ownerRepoForState(files, missing); owner != "" {
			t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CHAIN_TRIGGERED",
				fmt.Sprintf("담을 자리를 만들 저장소에 넘긴다: %s", owner), why, strings.Join(missing, " · "))
			return []StatelessRequest{t.stateChainRequest(owner, missing)}
		} else if why != "" {
			t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CHAIN_SKIPPED",
				"타입에서 임자를 못 찾았다 — 저장소 고르기로 물어본다", why, "")
		}
	}
	q := fmt.Sprintf("%s 에 %s 이(가) 필요하다. %s",
		t.targetRepo, strings.Join(missing, ", "), strings.TrimSpace(t.req.UserRequest))

	sub, cancel := context.WithTimeout(t.ctx, 30*time.Second)
	defer cancel()
	routed, err := t.orchestrator.insightClient.RouteRepos(sub, q)
	if err != nil {
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CHAIN_SKIPPED",
			"저장소 고르기가 답하지 않았다", err.Error(), strings.Join(missing, " · "))
		return nil
	}
	// 걸러낸 것은 까닭과 함께 남긴다. 조용히 비우면 「못 골랐다」 만 남아
	// 무엇을 보고 그랬는지 알 수 없다.
	var dropped []string
	for _, r := range routed {
		if r.RepoName == t.targetRepo || hasRepo(t.req.ParentRepos, r.RepoName) {
			dropped = append(dropped, r.RepoName+"(이미 거쳐 온 저장소)")
			continue
		}
		if !t.orchestrator.wsMgr.HasRepo(r.RepoName) {
			dropped = append(dropped, r.RepoName+"(사본이 없다)")
			continue
		}
		t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CHAIN_TRIGGERED",
			fmt.Sprintf("담을 자리를 만들 저장소에 넘긴다: %s", r.RepoName), "저장소 고르기가 골랐다", strings.Join(missing, " · "))
		return []StatelessRequest{t.stateChainRequest(r.RepoName, missing)}
	}
	// **왜 못 만들었는지 적는다.** 조용히 비우면 사람은 연쇄가 도는 줄 안다.
	why := "저장소 고르기가 아무것도 내놓지 않았다"
	if len(dropped) > 0 {
		why = "걸러낸 것: " + strings.Join(dropped, ", ")
	}
	t.orchestrator.logDeepTechnical(t.ctx, t.taskID, "CHAIN_SKIPPED",
		"담을 자리를 만들 저장소를 못 골랐다", why, strings.Join(missing, " · "))
	return nil
}

// stateChainRequest 는 그 저장소에 남길 일이다.
//
// **넘기는 쪽이 아는 것을 다 싣는다.** 부모는 이미 어느 파일·어느 계약·어떤
// 발행 목표인지 알아냈는데, 그것을 빼고 「gig_ceo_web 에서 …」 로 시작하는
// 산문만 넘겼다. 그래서 자식이 대상을 다시 고르고 엉뚱한 저장소를 뒤지다
// 넷 다 0줄로 죽었다.
//
// 첫 줄이 **할 일**이어야 한다 — 어디서 막혔는지는 배경으로 뒤에 둔다.
func (t *taskContext) stateChainRequest(owner string, missing []string) StatelessRequest {
	// **어느 메시지에 붙이는지까지 알려 준다.**
	//
	// 파일만 알려 줬더니 자식이 그 파일에서 타입을 못 찾고 없는 메시지를
	// 지어냈다(ConnectRequest — 진짜는 ReceivedRequest 이고 같은 폴더의
	// 다른 파일에 있다). 계약 파일은 서비스와 메시지를 나눠 두는 일이 흔하다.
	// **자리가 둘이다.** 필드는 메시지가 선언된 파일에, RPC 는 서비스가
	// 선언된 파일에 들어간다. 계약은 그 둘을 나눠 두는 일이 흔한데 자리를
	// 하나만 알려 주면 자식이 한쪽에 다 밀어 넣고 없는 메시지를 지어낸다.
	svcPath, msg := t.protoPath, t.stateType
	msgPath := ""
	if msg != "" {
		msgPath = protoFileDeclaring(t.orchestrator.wsMgr.RepoPath(owner), msg)
	}

	var where string
	switch {
	case msgPath != "" && svcPath != "" && msgPath != svcPath:
		where = fmt.Sprintf("고칠 자리\n  필드: %s 의 %s 메시지\n  RPC : %s 의 service\n펴내기: %s\n",
			msgPath, msg, svcPath, t.protoTarget)
	case msgPath != "":
		where = fmt.Sprintf("고칠 자리: %s (%s 메시지)\n펴내기: %s\n", msgPath, msg, t.protoTarget)
	case svcPath != "":
		where = fmt.Sprintf("고칠 자리: %s\n펴내기: %s\n", svcPath, t.protoTarget)
	}
	if msg != "" {
		where += fmt.Sprintf("**%s 는 이미 있다. 그것을 고쳐라 — 같은 이름으로 새로 만들지 마라.**\n", msg)
	}
	where += protoConvention(t.orchestrator.wsMgr.RepoPath(owner), msgPath, svcPath)
	// 이미 무엇이 있는지 보여 준다. 자식이 「이미 있는지 확인하지 못했다」 로
	// 죽지 않게 — 모델이 뒤져서 알아낼 일이 아니라 세어서 주면 되는 것이다.
	where += protoOutline(t.orchestrator.wsMgr.RepoPath(owner), msgPath, svcPath)
	return StatelessRequest{
		UserRequest: fmt.Sprintf(
			"%s 저장소에 이것을 더해라: %s\n%s"+
				"상태를 담을 필드와 그것을 바꾸는 길(RPC)을 함께 더한다.\n"+
				"**계약은 원본(.proto)만 고친다.** 펴낸 결과물(*.pb.go·생성된 .ts)은 손대지 마라 — "+
				"다음 발행 때 덮어써진다.\n\n"+
				"[배경] %s 에서 「%s」 를 만들려는데 담을 자리가 없어 막혔다.",
			owner, strings.Join(missing, ", "), where,
			t.targetRepo, strings.TrimSpace(t.req.UserRequest)),
		TargetRepo:   owner,
		AddsState:    true,
		Depth:        t.req.Depth - 1,
		ParentRepos:  append(t.req.ParentRepos, t.targetRepo),
		ParentTaskID: rootTaskID(t),
	}
}

// protoFileDeclaring 은 그 메시지를 선언한 .proto 파일을 준다.
func protoFileDeclaring(repoPath, message string) string {
	if repoPath == "" || message == "" {
		return ""
	}
	want := regexp.MustCompile(`(?m)^\s*message\s+` + regexp.QuoteMeta(message) + `\s*\{`)
	found := ""
	forEachProto(repoPath, func(rel, src string) {
		if found == "" && want.MatchString(src) {
			found = rel
		}
	})
	return found
}

// protoConvention 은 그 계약이 지켜 온 이름 관행을 세어서 알려 준다.
//
// 자식이 되풀이해 이 저장소에 없는 모양을 만들었다 — service ConnectService
// 를 새로 만들고(모든 꾸러미의 서비스는 Internal 하나뿐이다), 메시지를
// XxxRequest 접미형으로 지었다(이 계약은 RequestXxx 접두형이 201개,
// 접미형이 5개다). 새 서비스는 아무도 구현하지 않으므로 그대로 펴내면
// 부르는 쪽이 unimplemented 로 죽는다.
//
// 관행은 물어볼 것이 아니라 세면 되는 것이다.
func protoConvention(repoPath, msgPath, svcPath string) string {
	if repoPath == "" {
		return ""
	}
	dirs := map[string]bool{}
	for _, p := range []string{msgPath, svcPath} {
		if p != "" {
			dirs[path.Dir(p)] = true
		}
	}

	services := map[string]int{}
	pre, suf := 0, 0
	forEachProto(repoPath, func(rel, src string) {
		if len(dirs) > 0 && !dirs[path.Dir(rel)] {
			return
		}
		for _, m := range reServiceDecl.FindAllStringSubmatch(src, -1) {
			services[m[1]]++
		}
		for _, m := range reProtoDecl.FindAllStringSubmatch(src, -1) {
			if m[1] != "message" {
				continue
			}
			switch {
			case strings.HasPrefix(m[2], "Request"), strings.HasPrefix(m[2], "Response"):
				pre++
			case strings.HasSuffix(m[2], "Request"), strings.HasSuffix(m[2], "Response"):
				suf++
			}
		}
	})

	var out []string
	if len(services) == 1 {
		for name := range services {
			out = append(out, fmt.Sprintf("이 계약의 서비스는 %s 하나뿐이다 — **새 서비스를 만들지 말고 그 안에 rpc 를 더해라.** 새 서비스는 아무도 구현하지 않는다.", name))
		}
	} else if len(services) > 1 {
		var names []string
		for name := range services {
			names = append(names, name)
		}
		sort.Strings(names)
		out = append(out, fmt.Sprintf("이 계약의 서비스는 %s 다 — 그 가운데 하나에 rpc 를 더해라.", strings.Join(names, " · ")))
	}
	if pre+suf > 0 && pre > suf*2 {
		out = append(out, fmt.Sprintf("메시지 이름은 RequestXxx·ResponseXxx 접두형이다(%d개, 접미형 %d개).", pre, suf))
	}
	if len(out) == 0 {
		return ""
	}
	return "[이 계약의 관행]\n" + strings.Join(out, "\n") + "\n"
}
