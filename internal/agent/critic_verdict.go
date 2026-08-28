package agent

import (
	"regexp"
	"strings"
)

// 검토 결과를 **판정으로** 받는다. 비평가와 리뷰어가 같은 규칙을 쓴다.
//
// "위험을 찾아라" 라고만 시키면 9B 는 늘 찾아낸다. 실측으로 본 두 가지:
//
//   - enum 에 String() 을 더한 변경에 "Unknown 을 돌려주면 민감정보가 샐 수
//     있다" 는 막연한 걱정
//   - diff 와 아무 상관 없는 `slog.Logger` 이야기 (모델이 딴 데로 샜다)
//
// 둘 다 거절로 읽혀 작업이 통째로 버려졌다. **늘 무언가를 찾아내는 검토자는
// 없느니만 못하다.**
//
// 그래서 근거를 요구한다:
//  1. 파일·줄을 대야 한다
//  2. 그 파일이 **실제로 이번 diff 에 있어야** 한다 — 지어낸 자리는 근거가 아니다

var gateLocation = regexp.MustCompile(`[\w./-]+\.(go|ts|tsx|dart|sql|yaml|yml|prisma)(:\d+)?`)

// diff 의 `+++ b/path` 에서 바뀐 파일을 뽑는다.
var gateDiffFile = regexp.MustCompile(`(?m)^\+\+\+ b/(.+)$`)

type CriticVerdict struct {
	Blocking  bool
	Locations []string
	Raw       string
	Why       string
}

// ParseGateVerdict 는 검토 응답을 판정으로 바꾼다.
// marker 가 없으면 통과, 있어도 **이번 diff 안의** 자리를 못 대면 통과다.
func ParseGateVerdict(resp, diff, marker string) CriticVerdict {
	raw := strings.TrimSpace(resp)
	upper := strings.ToUpper(raw)

	if raw == "" {
		return CriticVerdict{Raw: raw, Why: "검토자가 아무 말도 하지 않았습니다 — 막지 않습니다."}
	}
	if !strings.Contains(upper, strings.ToUpper(marker)) {
		return CriticVerdict{Raw: raw, Why: "문제를 찾지 못했습니다."}
	}

	locs := uniqueStrings(gateLocation.FindAllString(raw, -1))
	if len(locs) == 0 {
		return CriticVerdict{
			Raw: raw,
			Why: "문제라 했지만 파일·줄을 대지 못했습니다 — 막연한 지적으로 보고 넘어갑니다.",
		}
	}

	// **이번 변경 안의 자리만 인정한다.** 모델이 diff 에 없는 파일을 들고
	// 오는 일이 실제로 있었다.
	changed := changedFileSet(diff)
	if len(changed) > 0 {
		var kept []string
		for _, l := range locs {
			if changed[strings.SplitN(l, ":", 2)[0]] {
				kept = append(kept, l)
			}
		}
		if len(kept) == 0 {
			return CriticVerdict{
				Raw: raw,
				Why: "지적한 자리(" + strings.Join(locs, ", ") + ")가 이번 변경에 없습니다 — 넘어갑니다.",
			}
		}
		locs = kept
	}

	return CriticVerdict{
		Blocking:  true,
		Locations: locs,
		Raw:       raw,
		Why:       "문제를 짚었습니다: " + strings.Join(locs, ", "),
	}
}

func ParseCriticVerdict(resp, diff string) CriticVerdict {
	return ParseGateVerdict(resp, diff, "RISK_DETECTED")
}

func ParseReviewerVerdict(resp, diff string) CriticVerdict {
	return ParseGateVerdict(resp, diff, "FEEDBACK")
}

func changedFileSet(diff string) map[string]bool {
	out := map[string]bool{}
	for _, m := range gateDiffFile.FindAllStringSubmatch(diff, -1) {
		if p := strings.TrimSpace(m[1]); p != "" {
			out[p] = true
		}
	}
	return out
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
