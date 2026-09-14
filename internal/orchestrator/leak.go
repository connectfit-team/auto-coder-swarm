package orchestrator

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/guard"
)

// 도구가 사는 세상이 제품 코드로 새면 안 된다.
//
// "고용주웹에서 연결보류 기능을 추가" 가 승인 대기까지 갔는데, 넣은 코드가
// 이것이었다(W-76095).
//
//	func (s *ConnectedCEODeviceService) EnsureServiceIsRunning() error {
//	    endpoint := "http://127.0.0.1:8000/v1/chat/completions"
//	    return s.checkEndpoint(endpoint)
//	}
//
// 8000 번은 **이 시스템이 쓰는 vLLM** 이다. 모델이 제 환경에서 본 주소를
// 제품 코드에 적은 것이다. 연결보류와는 아무 상관이 없다.
//
// 값 추가 흐름에는 「시킨 일인가」 를 보는 관문이 있는데(CheckAlignment)
// 결함 흐름에는 없었다. 검토자가 반대해도 「사람 판단으로 넘긴다」 로
// 승인 대기까지 간다 — 그 길에 관문이 없으면 사람이 제목만 보고 통과시킨다.

// 이 기계의 포트들. 제품 코드에 나올 이유가 없다.
var toolPorts = []string{"8000", "8005", "8006", "8007", "8082", "6333", "6334", "11434"}

var localAddrRe = regexp.MustCompile(`(?i)https?://(127\.0\.0\.1|localhost|0\.0\.0\.0)(:\d+)?`)

// CheckToolLeak 은 넣은 코드가 도구의 환경을 베꼈는지 본다.
func CheckToolLeak(diff string) []guard.Violation {
	var bad []guard.Violation
	var hits []string
	for _, line := range strings.Split(diff, "\n") {
		if !strings.HasPrefix(line, "+") || strings.HasPrefix(line, "+++") {
			continue
		}
		m := localAddrRe.FindString(line)
		if m == "" {
			continue
		}
		// 시험 파일에서 제 서버를 띄우는 것은 정상이다.
		if strings.Contains(strings.ToLower(line), "test") {
			continue
		}
		hits = append(hits, strings.TrimSpace(line))
	}
	if len(hits) > 0 {
		if len(hits) > 5 {
			hits = hits[:5]
		}
		bad = append(bad, guard.Violation{
			Why:      "이 기계의 주소를 제품 코드에 넣었다 — 도구가 사는 세상이지 제품의 것이 아니다",
			Evidence: hits,
		})
	}

	// 포트만 적은 것도 본다(주소 없이 ":8000" 꼴).
	var ports []string
	for _, line := range strings.Split(diff, "\n") {
		if !strings.HasPrefix(line, "+") || strings.HasPrefix(line, "+++") {
			continue
		}
		for _, p := range toolPorts {
			if strings.Contains(line, ":"+p) && !strings.Contains(strings.ToLower(line), "test") {
				ports = append(ports, fmt.Sprintf("%s — %s", p, strings.TrimSpace(line)))
				break
			}
		}
	}
	if len(ports) > 0 {
		if len(ports) > 5 {
			ports = ports[:5]
		}
		bad = append(bad, guard.Violation{
			Why:      "이 기계가 쓰는 포트를 제품 코드에 넣었다",
			Evidence: ports,
		})
	}
	return bad
}
