package orchestrator

import (
	"os/exec"

	"github.com/connectfit-team/auto-coder-swarm/internal/guard"
)

// 값 하나를 더하는 일에만 해당하는 어긋남을 본다.
//
// 어느 흐름에나 해당하는 것(생성물·설정·비밀·보호된 가지)은 미는 자리에서
// 본다(internal/guard). 여기서는 값 추가에만 해당하는 둘을 본다 —
// 더하기만 해야 한다는 것, 그리고 요청이 지목한 저장소만 고쳐야 한다는 것.
//
// 미는 자리에도 문이 있는데 여기서 또 보는 까닭: 밀기 전에 막으면 사람에게
// 왜인지 더 잘 설명할 수 있고, 헛되게 커밋하지 않는다.

// AlignmentInput 은 판단에 필요한 것이다.
type AlignmentInput struct {
	Repo    string
	Value   string   // 더하는 값
	Diff    string   // git diff
	Named   []string // 요청이 지목한 저장소. 비면 지목하지 않았다
	Blocker bool     // proto 처럼 지목 밖이라도 함께 가야 하는 저장소
}

// CheckAlignment 는 이 편집이 시킨 일인지 본다. 빈 목록이면 맞는 것이다.
func CheckAlignment(in AlignmentInput) []guard.Violation {
	out := guard.BeforePush(in.Diff, "")
	out = append(out, guard.InsertOnly(in.Diff)...)
	if len(in.Named) > 0 && !in.Blocker && !hasRepo(in.Named, in.Repo) {
		out = append(out, guard.Violation{
			Why:      "요청이 지목하지 않은 저장소를 고쳤다 — 지목한 것은 " + joinRepos(in.Named),
			Evidence: guard.ChangedFiles(in.Diff),
		})
	}
	return out
}

// AlignmentNote 는 어긋난 것들을 사람이 읽을 글로 만든다.
func AlignmentNote(bad []guard.Violation) string { return guard.Note(bad) }

// stagedDiff 는 지금 워크트리의 편집을 준다.
//
// 새로 만든 파일도 봐야 하므로 먼저 담는다(git add -A). 바로 다음 단계가
// 어차피 담아서 커밋하고, 어긋나 멈추면 이 워크트리는 지워진다.
func stagedDiff(path string) string {
	if err := exec.Command("git", "-C", path, "add", "-A").Run(); err != nil {
		return ""
	}
	out, err := exec.Command("git", "-C", path, "diff", "--cached", "HEAD").Output()
	if err != nil {
		return ""
	}
	return string(out)
}

func hasRepo(named []string, repo string) bool {
	for _, n := range named {
		if equalFoldTrim(n, repo) {
			return true
		}
	}
	return false
}
