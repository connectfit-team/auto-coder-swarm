package orchestrator

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// 같은 값을 두 번 요청하면 저장소마다 PR 이 하나씩 더 열린다. 브랜치 이름에
// 작업 번호가 들어가 매번 달라서 git 은 이것을 겹침으로 보지 못한다.
// 열려 있는 PR 의 브랜치 앞머리를 보고 판단한다.

type openPR struct {
	URL     string `json:"url"`
	Branch  string `json:"headRefName"`
	IsDraft bool   `json:"isDraft"`
}

// existingVariantPR 은 이 저장소에 같은 값을 더하는 PR 이 이미 열려 있으면
// 그 주소를 준다. 조회에 실패하면 빈 문자열이다 — 못 본 것을 있다고 하지 않는다.
func existingVariantPR(repo, value string) string {
	out, err := exec.Command("gh", "pr", "list",
		"--repo", "connectfit-team/"+repo,
		"--state", "open", "--limit", "100",
		"--json", "url,headRefName,isDraft").Output()
	if err != nil {
		return ""
	}
	var prs []openPR
	if json.Unmarshal(out, &prs) != nil {
		return ""
	}
	prefix := fmt.Sprintf("feat/add-%s-", value)
	for _, p := range prs {
		if strings.HasPrefix(p.Branch, prefix) {
			return p.URL
		}
	}
	return ""
}
