package orchestrator

import (
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/guard"
)

var (
	reAddedRPC      = regexp.MustCompile(`^\s*rpc\s+(\w+)\s*\(`)
	reStateFieldAny = regexp.MustCompile(`(?i)^\s*(?:repeated\s+|optional\s+)?[\w.]+\s+(\w*(?:status|state))\s*=\s*\d+\s*;`)
)

// stateChangeNeedsAnRPC 는 **담을 자리만 만들고 바꾸는 길을 안 낸 것**을 잡는다.
//
// 연쇄가 넘기는 쪽지에 이렇게 적혀 있다.
//
//	상태를 담을 필드와 그것을 바꾸는 길(RPC)을 함께 더한다.
//
// 그런데 필드만 넣고 끝낸 일이 두 번 있었고, **그때마다 관문이 아무 말도 안
// 했다.** 판정기만 뒤늦게 잡았다.
//
//	W-11906  비평가 되돌이에서 RPC 를 잃고 승인 대기까지 감
//	W-23965  「계획에 없는 파일」 관문이 service 파일을 지워 RPC 가 사라짐
//
// 둘 다 관문을 다 지났다. 필드가 들어간 것만 보면 멀쩡해 보이기 때문이다.
// 쓰는 쪽에서 보면 **읽을 수는 있는데 바꿀 수가 없는 계약**이다.
//
// 담을 자리를 만드는 일(AddsState)일 때만 본다. 여느 수정에는 해당 없다.
func stateChangeNeedsAnRPC(repoPath, diff string) []guard.Violation {
	addedField, addedRPC := "", false
	for _, line := range strings.Split(diff, "\n") {
		if !strings.HasPrefix(line, "+") || strings.HasPrefix(line, "+++") {
			continue
		}
		body := line[1:]
		if reAddedRPC.MatchString(body) {
			addedRPC = true
		}
		if addedField == "" {
			if m := reStateFieldAny.FindStringSubmatch(body); m != nil {
				addedField = m[1]
			}
		}
	}
	// 필드를 넣지도 않았으면 이 검사가 할 말이 없다 — 다른 관문이 말한다.
	if addedField == "" || addedRPC {
		return nil
	}

	ev := []string{
		"넘긴 쪽지는 「상태를 담을 필드와 **그것을 바꾸는 길(RPC)**을 함께 더한다」 였다.",
		"필드만 있으면 쓰는 쪽은 읽을 수는 있어도 바꿀 수가 없다 — 반쪽이다.",
	}
	// **어떤 rpc 를 어떤 모양으로 더할지까지 보여 준다.** 「더해라」 만으로는
	// 이름을 지어내고, 이 계약의 이름 버릇과 어긋난다.
	if sk := rpcSkeleton(repoPath, addedField); sk != "" {
		ev = append(ev, sk)
	}
	if s := serviceHint(repoPath, protoFilesInDiff(diff)); s != "" {
		ev = append(ev, s)
	}
	return []guard.Violation{{
		Why:      fmt.Sprintf("%s 를 넣었는데 그것을 바꾸는 rpc 가 없다", addedField),
		Evidence: ev,
	}}
}

// rpcSkeleton 은 더할 rpc 의 **모양**을 이 계약의 이름 버릇대로 지어 보여 준다.
//
// 필드 `pending_status` 면 `UpdatePendingStatus` 다. 접두형 계약이면
// `RequestUpdatePendingStatus`·`ResponseUpdatePendingStatus` 로 짝을 짓는다.
func rpcSkeleton(repoPath, field string) string {
	camel := ""
	for _, part := range strings.Split(field, "_") {
		if part == "" {
			continue
		}
		camel += strings.ToUpper(part[:1]) + part[1:]
	}
	if camel == "" {
		return ""
	}
	op := "Update" + camel
	req, res := "Request"+op, "Response"+op
	if pre, suf := nameHabit(repoPath); suf > pre {
		req, res = op+"Request", op+"Response"
	}
	return fmt.Sprintf("**이 모양으로 더한다** — 먼저 메시지 둘:\n"+
		"```proto\nmessage %s {\n  // 무엇을 바꿀지 가리키는 필드와 새 값\n}\n\nmessage %s {\n}\n```\n"+
		"그리고 service 안에:\n```proto\n  rpc %s(%s) returns (%s);\n```",
		req, res, op, req, res)
}

// CheckStateNeedsRPC 는 채점기가 부를 수 있게 연 것이다.
//
// 이 관문은 CheckProtoChange **바깥**에 있다(handOver 가 t.req.AddsState 일 때만
// 부른다). 채점기가 운영과 같은 것을 보려면 따로 불러야 한다 — 안 그러면
// 모형이 관문에 안 걸리고, 그것을 모르고 「고쳤다」 고 센다.
func CheckStateNeedsRPC(repoPath, diff string) []guard.Violation {
	return stateChangeNeedsAnRPC(repoPath, diff)
}

// serviceHint 는 rpc 를 어디에 더하면 되는지 **보여 준다.**
// 「더해라」 라고만 하지 않는다 — 이 세션 내내 같은 교훈이었다.
//
// **이번에 고치는 파일을 먼저 본다.** 저장소에서 아무 service 나 집으면 엉뚱한
// 파일로 보낸다 — 첫 판에서 `connect.service.proto` 를 고치는 중인데
// `ceo.service.proto` 를 가리켰다. 자리를 잘못 짚어 주는 것은 이 세션 내내
// 싸운 실패 모양 그대로다.
func serviceHint(repoPath string, touched []string) string {
	if repoPath == "" {
		return ""
	}
	find := func(rel string) (string, bool) {
		for _, line := range strings.Split(protoAtHead(repoPath, rel), "\n") {
			if m := protoServiceRe.FindStringSubmatch(line); m != nil {
				return m[1], true
			}
		}
		return "", false
	}
	// 1) 이번에 고치는 파일에 service 가 있으면 그것.
	for _, rel := range touched {
		if name, ok := find(rel); ok {
			return hintLine(name, rel)
		}
	}
	// 2) 없으면 같은 폴더의 이웃. 관행은 꾸러미의 것이다.
	dirs := map[string]bool{}
	for _, rel := range touched {
		dirs[path.Dir(filepath.ToSlash(rel))] = true
	}
	var name, rel string
	forEachProto(repoPath, func(r string, _ string) {
		if name != "" || (len(dirs) > 0 && !dirs[path.Dir(r)]) {
			return
		}
		if n, ok := find(r); ok {
			name, rel = n, r
		}
	})
	if name == "" {
		return ""
	}
	return hintLine(name, rel)
}

func hintLine(name, rel string) string {
	return fmt.Sprintf("이 계약에는 `service %s` 가 %s 에 있다 — **그 안에** rpc 를 더해라. "+
		"새 service 는 아무도 구현하지 않는다.", name, rel)
}
