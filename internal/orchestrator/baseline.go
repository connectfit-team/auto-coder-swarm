package orchestrator

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// 검증 명령은 **기준선을 통과하는 가장 엄한 것**으로 고른다.
//
// 새 워크트리에는 의존성도 생성물도 없어서 무슨 코드를 쓰든 검증이 실패할 수
// 있다. 그래서 손대기 전에 한 번 돌려 본다. 그런데 실패하면 곧바로 작업을
// 죽이고 있었고, 그 결과 **원래 깨져 있는 시험 파일 하나가 저장소 전체를
// 영원히 잠갔다** — ceo 의 connect_handler_disconnect_test.go 가 옛 시그니처를
// 부르고 있어 "연결보류 기능 추가" 가 계획 단계에서 죽었다(W-80655).
// 그 시험은 요청과 아무 상관이 없다.
//
// 검증 관문은 **절대 상태가 아니라 변화**를 재야 한다. 엄한 명령이 이미
// 깨져 있으면 한 칸 물러선 명령으로 재고, 물러섰다는 것을 사람에게 알린다.

// baselineChoice 는 고른 명령과 그 까닭이다.
type baselineChoice struct {
	Command string
	// 엄한 명령을 못 쓴 까닭. 비어 있으면 엄한 것을 그대로 쓴다.
	SteppedDown string
	// 어느 명령도 통과하지 못했을 때의 출력.
	FailedOutput string
}

const baselineOutputLines = 15

// pickBaselineCommand 는 기준선을 통과하는 가장 엄한 명령을 고른다.
func (t *taskContext) pickBaselineCommand(strict string, weaker []string) baselineChoice {
	ladder := append([]string{strict}, weaker...)
	var lastOut string
	for i, cmd := range ladder {
		if isNoCommand(cmd) {
			continue
		}
		out, err := shellCmd(t.ctx, t.repoPath, cmd).CombinedOutput()
		if err == nil {
			c := baselineChoice{Command: cmd}
			if i > 0 {
				c.SteppedDown = fmt.Sprintf("%q 는 손대기 전부터 실패한다 — %q 로 잰다",
					ladder[i-1], cmd)
			}
			return c
		}
		lastOut = string(out)
	}
	out := clipLines(lastOut, baselineOutputLines)
	// **도구가 없는 것과 코드가 안 되는 것은 다르다.**
	//
	// `flutter: command not found` 를 "이 저장소는 빌드되지 않는다" 로
	// 보고했다(W-58890). 사람이 고칠 것은 저장소가 아니라 PATH 다.
	if tool := missingTool(lastOut); tool != "" {
		out = tool + " 가 이 기계에 없다(PATH). 저장소 문제가 아니다.\n" + out
	}
	return baselineChoice{FailedOutput: out}
}

var notFoundRe = regexp.MustCompile(`([\w.\-/]+): (?:command not found|not found)`)

// missingTool 은 "그런 명령이 없다" 는 출력에서 도구 이름을 뽑는다.
func missingTool(out string) string {
	if m := notFoundRe.FindStringSubmatch(out); m != nil {
		return m[1]
	}
	return ""
}

// clipLines 는 앞 n 줄만 남긴다. 사람이 읽을 만큼만 보여 준다.
func clipLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) <= n {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[:n], "\n") + fmt.Sprintf("\n… 그 밖에 %d줄", len(lines)-n)
}

// steppedDownNote 는 무엇을 못 검사했는지 PR 머리에 적는다.
// 적지 않으면 사람은 검증이 다 돌았다고 믿는다.
func steppedDownNote(msg string) string {
	if msg == "" {
		return ""
	}
	return "> **검증을 한 칸 물러섰다.** " + msg + "\n>\n" +
		"> 이 저장소는 손대기 전부터 엄한 명령이 실패한다(대개 원래 깨져 있는\n" +
		"> 시험 파일이다). 그래서 이 PR 은 그 검사를 **받지 않았다** —\n" +
		"> 시험 파일이 컴파일되는지는 사람이 봐야 한다.\n\n"
}

var _ = context.Background
