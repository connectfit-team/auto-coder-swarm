package orchestrator

import (
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

// 이 기계의 포트들. 제품 코드에 **주소와 함께** 나올 이유가 없다.
const toolPorts = `8000|8005|8006|8007|8082|6333|6334|11434`

var (
	// 이 기계를 가리키는 주소. 포트가 없어도 잡는다.
	localAddrRe = regexp.MustCompile(`(?i)https?://(127\.0\.0\.1|localhost|0\.0\.0\.0)(:\d+)?`)

	// 어느 host 든 이 기계의 포트를 달고 있으면 잡는다.
	//
	// 맨 포트만 보면 안 된다 — `timeout:8000` 같은 멀쩡한 코드가 걸린다.
	// 앞에 host 가 붙어야 주소다.
	hostPortRe = regexp.MustCompile(`(?i)(?://|@)[\w.\-]+:(` + toolPorts + `)\b`)

	// 이 시스템의 도구 이름. 제품 코드에 나올 이유가 없다.
	toolNameRe = regexp.MustCompile(`(?i)\b(vllm|qdrant|ollama|embedding-master)\b|/v1/chat/completions`)

	// 시험 파일은 제 서버를 띄우는 것이 정상이다. **줄이 아니라 파일로** 가른다.
	// 줄에 "test" 가 있으면 넘기던 때에는 `/v1/latest` 도 통과했다.
	testPathRe = regexp.MustCompile(`(?i)(_test\.go|\.test\.|\.spec\.|(^|/)tests?/|__tests__/|/testdata/|\.stories\.)`)
)

// CheckToolLeak 은 넣은 코드가 도구의 환경을 베꼈는지 본다.
func CheckToolLeak(diff string) []guard.Violation {
	var addrs, names []string
	file := ""
	inTest := false
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "+++ ") {
			file = strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(line, "+++ ")), "b/")
			inTest = testPathRe.MatchString(file)
			continue
		}
		if !strings.HasPrefix(line, "+") || inTest {
			continue
		}
		body := line[1:]
		if m := localAddrRe.FindString(body); m != "" {
			addrs = append(addrs, file+": "+strings.TrimSpace(body))
			continue
		}
		if m := hostPortRe.FindString(body); m != "" {
			addrs = append(addrs, file+": "+strings.TrimSpace(body))
			continue
		}
		if m := toolNameRe.FindString(body); m != "" {
			names = append(names, file+": "+strings.TrimSpace(body))
		}
	}

	var bad []guard.Violation
	if len(addrs) > 0 {
		bad = append(bad, guard.Violation{
			Why:      "이 기계의 주소를 제품 코드에 넣었다 — 도구가 사는 세상이지 제품의 것이 아니다",
			Evidence: clipList(addrs),
		})
	}
	if len(names) > 0 {
		bad = append(bad, guard.Violation{
			Why:      "이 시스템의 도구 이름을 제품 코드에 넣었다",
			Evidence: clipList(names),
		})
	}
	return bad
}

func clipList(xs []string) []string {
	if len(xs) > 5 {
		return xs[:5]
	}
	return xs
}
