package orchestrator

import "strings"

// 모델은 "명령이 없다" 를 말로 낸다.
//
// N/A · none · 없음 · - · null 같은 것을 그대로 셸에 넘기면 `sh: N/A: not
// found` 로 실패하고, 그 실패가 코드나 환경 탓으로 보고된다. 빈 명령도 마찬가지로
// 위험한데 방향이 반대다 — `bash -c ""` 는 exit 0 이라 아무것도 검증하지 않고
// "성공" 으로 지나간다.
var noCommandWords = map[string]bool{
	"n/a": true, "na": true, "none": true, "nil": true, "null": true,
	"no": true, "-": true, "--": true, "없음": true, "해당없음": true,
	"not applicable": true, "unknown": true, "n/a.": true, "skip": true,
	"true": true, ":": true,
}

// isNoCommand 는 그것이 명령이 아니라 "없다" 는 말인지 본다.
func isNoCommand(cmd string) bool {
	c := strings.ToLower(strings.TrimSpace(cmd))
	if c == "" {
		return true
	}
	return noCommandWords[c]
}
