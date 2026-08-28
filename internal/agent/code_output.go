package agent

import "strings"

// 모델이 낸 것에서 **코드만** 꺼낸다.
//
// 프롬프트에 "코드만 내라, 대화체를 붙이지 마라" 고 적어도 지켜지지 않는다.
// 실측으로 코더가 파일 첫 줄에 ```go 를 그대로 써서 빌드가 이렇게 죽었다:
//
//	internal/domain/tag.go:1:1: expected 'package', found ``
//
// 그리고 그 실패는 자가치유를 세 번, 계획을 세 번 다시 돌게 만들었다 —
// 원인은 늘 같은 세 글자였다. 형식은 부르는 쪽이 지켜 낸다.
func CleanCodeOutput(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return s
	}

	// 펜스가 있으면 **가장 긴 블록**을 코드로 본다. 설명 안에 짧은 예시가
	// 섞여 있어도 본문을 고르게 된다.
	if best := longestFenced(s); best != "" {
		return strings.TrimSpace(best)
	}

	// 펜스가 없으면 앞뒤의 군말만 걷는다. 코드는 대개 package/import/주석으로
	// 시작하므로, 그 앞에 붙은 산문을 떼어 낸다.
	return strings.TrimSpace(stripLeadingProse(s))
}

func longestFenced(s string) string {
	best := ""
	for i := 0; ; {
		open := strings.Index(s[i:], "```")
		if open < 0 {
			break
		}
		open += i
		// 여는 줄의 나머지는 언어 표시다(```go).
		nl := strings.IndexByte(s[open:], '\n')
		if nl < 0 {
			break
		}
		bodyStart := open + nl + 1
		close := strings.Index(s[bodyStart:], "```")
		if close < 0 {
			// 닫히지 않았으면 끝까지가 본문이다 — 모델이 자주 빠뜨린다.
			if body := s[bodyStart:]; len(body) > len(best) {
				best = body
			}
			break
		}
		if body := s[bodyStart : bodyStart+close]; len(body) > len(best) {
			best = body
		}
		i = bodyStart + close + 3
	}
	return best
}

// 코드가 시작하는 줄. 여기 없는 언어를 만나면 그때 더한다.
var codeStarters = []string{
	"package ", "import ", "//", "/*", "#!", "#include",
	"const ", "var ", "func ", "type ", "class ", "export ", "using ",
	"from ", "def ", "@", "<?php", "<!DOCTYPE", "<html",
}

func stripLeadingProse(s string) string {
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		t := strings.TrimSpace(ln)
		if t == "" {
			continue
		}
		for _, p := range codeStarters {
			if strings.HasPrefix(t, p) {
				return strings.Join(lines[i:], "\n")
			}
		}
		// 첫 비어 있지 않은 줄이 코드처럼 안 보이면 산문이다. 계속 본다.
		if i > 6 {
			break // 앞부분만 본다 — 코드 한가운데를 자르면 안 된다
		}
	}
	return s
}
