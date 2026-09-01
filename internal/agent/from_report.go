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
	var out []FileChange
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

		out = append(out, FileChange{
			FilePath:    path,
			Description: reason,
			Instructions: "아래 줄이 이 문제의 원인이다. 이 줄만 고쳐라.\n" +
				"[문제의 줄]\n" + cause + "\n[왜 문제인가]\n" + reason,
		})
	}
	return out
}

func firstGroup(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return strings.TrimSpace(strings.Trim(strings.TrimSpace(m[1]), "`"))
	}
	return ""
}
