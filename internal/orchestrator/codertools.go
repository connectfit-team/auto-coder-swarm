package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
	"github.com/connectfit-team/auto-coder-swarm/internal/agentloop"
	"github.com/connectfit-team/auto-coder-swarm/internal/mcpclient"
)

// 도구 결과가 쓸 수 있는 토큰. 창이 16,384 라 하나가 길면 궤적을 밀어낸다.
const coderToolTokens = 900

// 찾기 결과 줄 수. 더 보려면 좁혀서 다시 묻게 한다.
const searchMaxLines = 40

// coderTools 는 코더가 **한 턴 안에서** 쓸 도구다.
//
// 관문은 본래 빌드·비평·검토를 다 지난 맨 끝에서 운다. #206 이 코딩 직후로
// 당겼지만 그것도 기계가 정해진 자리에서 돌리는 것이다. 모델이 스스로 부를 수
// 있으면 **고치는 자리에서 바로** 본다.
//
// 판정은 handOver 와 **같은 gateChecks** 를 쓴다. 두 곳에서 따로 판정하면
// 도구가 통과라 한 것을 관문이 막아 모델은 고칠 길이 없다(#205).
func (t *taskContext) coderTools() ([]mcpclient.Tool, agentloop.Dispatch) {
	tools := []mcpclient.Tool{
		{
			Name:        "check",
			Description: "지금 고친 것이 관문을 지나는지 본다. 고치고 나면 불러라.",
			Schema:      json.RawMessage(`{"type":"object","properties":{}}`),
		},
		{
			Name:        "read_file",
			Description: "저장소 안 파일을 읽는다. 지어내지 말고 읽어라.",
			Schema:      json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}`),
		},
		{
			Name:        "edit_file",
			Description: "파일을 고친다. 무엇을 어떻게 바꿀지 또렷하게 적어라.",
			Schema: json.RawMessage(`{"type":"object","properties":{` +
				`"path":{"type":"string"},"instructions":{"type":"string"}},` +
				`"required":["path","instructions"]}`),
		},
		{
			Name:        "search_code",
			Description: "저장소에서 글자를 찾는다. 이름이 이미 있는지 볼 때 쓴다.",
			Schema: json.RawMessage(`{"type":"object","properties":{` +
				`"query":{"type":"string"},"glob":{"type":"string","description":"예: *.proto"}},` +
				`"required":["query"]}`),
		},
	}

	dispatch := func(ctx context.Context, name string, args map[string]any) (string, error) {
		switch name {
		case "check":
			return t.checkNow(), nil
		case "read_file":
			p, _ := args["path"].(string)
			return readRepoFile(t.repoPath, p)
		case "edit_file":
			p, _ := args["path"].(string)
			how, _ := args["instructions"].(string)
			return t.editViaCoder(ctx, p, how)
		case "search_code":
			q, _ := args["query"].(string)
			g, _ := args["glob"].(string)
			return searchRepo(ctx, t.repoPath, q, g)
		}
		return "", fmt.Errorf("%s 라는 도구는 없다", name)
	}
	return tools, dispatch
}

// editViaCoder 는 **있던 편집 기계를 그대로 쓴다.**
//
// #SEARCH/REPLACE 와 자리 찾기 사다리는 오래 다듬은 것이다. 루프는 「무엇을
// 할지」 를 정하고, 「어떻게 고칠지」 는 코더가 한다.
func (t *taskContext) editViaCoder(ctx context.Context, rel, how string) (string, error) {
	full, err := insideRepo(t.repoPath, rel)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(how) == "" {
		return "", fmt.Errorf("무엇을 고칠지 적지 않았다")
	}
	if _, err := t.coder.ModifyFile(ctx, full, how); err != nil {
		// 까닭이 이미 또렷하다(「원문에 없는 내용을 찾으라고 했다」 + 가장
		// 비슷한 자리). 그대로 돌려주면 다음 턴에 고친다.
		return "", err
	}
	t.noteApproxRung(rel)
	t.noteStrippedComments(rel)
	return "고쳤다. check 로 관문을 보라.", nil
}

// checkNow 는 지금 작업 트리에 관문을 돌린다.
func (t *taskContext) checkNow() string {
	diff := t.currentDiff()
	if strings.TrimSpace(diff) == "" {
		return "아직 고친 것이 없다. 먼저 고치고 다시 불러라."
	}
	bad := t.gateChecks(diff)
	if len(bad) == 0 {
		return "관문을 지난다."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "관문에 걸린다 (%d가지). 아래 자리만 고쳐라 — 처음부터 다시 쓰지 마라.\n\n", len(bad))
	for _, g := range bad {
		fmt.Fprintf(&b, "## %s\n%s\n\n", g.label, g.feedback)
	}
	out, _ := agent.ClipToTokens(b.String(), coderToolTokens)
	return out
}

// readRepoFile 은 저장소 안쪽만 읽는다.
func readRepoFile(repoPath, rel string) (string, error) {
	full, err := insideRepo(repoPath, rel)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(full)
	if err != nil {
		return "", fmt.Errorf("%s 를 읽지 못했다: %w", rel, err)
	}
	out, cut := agent.ClipToTokens(string(b), coderToolTokens)
	if cut {
		out += "\n…(잘렸다 — search_code 로 좁혀서 찾아라)"
	}
	return out, nil
}

// searchRepo 는 git grep 으로 찾는다. 추적하지 않는 파일은 안 본다.
func searchRepo(ctx context.Context, repoPath, query, glob string) (string, error) {
	if repoPath == "" || strings.TrimSpace(query) == "" {
		return "", fmt.Errorf("찾을 말이 없다")
	}
	argv := []string{"-C", repoPath, "grep", "-n", "--fixed-strings", query}
	if glob != "" {
		argv = append(argv, "--", glob)
	}
	out, err := exec.CommandContext(ctx, "git", argv...).Output()
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		// git grep 은 못 찾으면 1 로 끝난다 — 오류가 아니다.
		return fmt.Sprintf("%q 는 없다.", query), nil
	}
	if err != nil && len(lines) == 0 {
		return "", fmt.Errorf("찾지 못했다: %w", err)
	}
	more := ""
	if len(lines) > searchMaxLines {
		more = fmt.Sprintf("\n…그 밖에 %d줄 더 있다. 좁혀서 다시 물어라.", len(lines)-searchMaxLines)
		lines = lines[:searchMaxLines]
	}
	res, _ := agent.ClipToTokens(strings.Join(lines, "\n")+more, coderToolTokens)
	return res, nil
}

// insideRepo 는 저장소 바깥으로 나가는 경로를 막는다.
func insideRepo(repoPath, rel string) (string, error) {
	if repoPath == "" {
		return "", fmt.Errorf("저장소가 없다")
	}
	if rel == "" {
		return "", fmt.Errorf("경로가 없다")
	}
	full := filepath.Clean(filepath.Join(repoPath, rel))
	root := filepath.Clean(repoPath)
	if full != root && !strings.HasPrefix(full, root+string(filepath.Separator)) {
		return "", fmt.Errorf("저장소 바깥은 못 읽는다: %s", rel)
	}
	return full, nil
}
