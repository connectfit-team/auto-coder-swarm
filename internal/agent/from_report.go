package agent

import (
	"regexp"
	"strings"
)

// CIE 가 고장 질문에 내는 답의 모양:
//
//	**`src/lib/server/workplace/workplace.ts`**
//	원인: lt: new Date(new Date().getFullYear(), new Date().getMonth() + 1, 0)
//	이유: lt 가 말일 00:00 이라 말일 데이터가 생략된다
var (
	reportFileRe   = regexp.MustCompile("(?m)^\\s*\\*\\*`([^`]+)`\\*\\*\\s*$")
	reportCauseRe  = regexp.MustCompile(`(?m)^\s*원인\s*:\s*(.+)$`)
	reportReasonRe = regexp.MustCompile(`(?m)^\s*이유\s*:\s*(.+)$`)
	reportConfRe   = regexp.MustCompile(`(?m)^\s*확신\s*:\s*(.+)$`)
)

// PlanFromDefectReport 는 분석이 이미 짚은 파일·줄을 그대로 계획으로 만든다.
//
// **아는 것을 다시 알아맞히게 하지 않는다.** 분석은 파일과 문제의 줄을 짚어
// 주는데, 그걸 다시 모델에게 넘겨 "어느 파일을 고칠까" 를 묻고 있었다. 그
// 되물음에서 엉뚱한 화면 파일로 새는 일이 잦았다.
//
// 원인 줄이 없으면 nil 을 준다 — 그때는 평소대로 모델에게 계획을 맡긴다.
func PlanFromDefectReport(analysis string) []FileChange {
	heads := reportFileRe.FindAllStringSubmatchIndex(analysis, -1)
	if len(heads) == 0 {
		return nil
	}
	var strong, weak []FileChange
	for i, h := range heads {
		path := strings.TrimSpace(analysis[h[2]:h[3]])
		end := len(analysis)
		if i+1 < len(heads) {
			end = heads[i+1][0]
		}
		body := analysis[h[1]:end]

		cause := firstGroup(reportCauseRe, body)
		if cause == "" || strings.Contains(body, "[무관]") {
			continue
		}
		reason := firstGroup(reportReasonRe, body)

		fc := FileChange{
			FilePath:    path,
			Description: reason,
			Instructions: "아래 줄이 이 문제의 원인이다. 이 줄만 고쳐라.\n" +
				"[문제의 줄]\n" + cause + "\n[왜 문제인가]\n" + reason,
		}
		if strings.Contains(firstGroup(reportConfRe, body), "높") {
			strong = append(strong, fc)
		} else {
			weak = append(weak, fc)
		}
	}

	// **확신이 높은 것만 고친다.**
	//
	// 분석에게 "관련 있어 보이는 코드를 하나 뽑아라" 고 하면 어느 파일에서든
	// 뭔가를 뽑아 온다 — 실측으로 파일 8개 중 7개에서 원인을 만들어 냈고,
	// 그 계획대로 고치니 정작 진짜 자리는 안 고치고 엉뚱한 파일 넷이 바뀌었다.
	// 확신이 높은 것이 하나도 없을 때만 나머지를 쓴다.
	if len(strong) > 0 {
		return strong
	}
	return weak
}

func firstGroup(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return strings.TrimSpace(strings.Trim(strings.TrimSpace(m[1]), "`"))
	}
	return ""
}
