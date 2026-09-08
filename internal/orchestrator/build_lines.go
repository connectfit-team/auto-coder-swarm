package orchestrator

import "strings"

// buildErrorLines 는 빌드 출력에서 오류 줄만 뽑는다.
//
// 나아지는지 세려면 셀 수 있는 것이 있어야 한다. 오류 줄 수가 그것이다 —
// 경고와 빈 줄, 꾸러미 머리(`# pkg/...`)는 세지 않는다.
func buildErrorLines(out string) []string {
	var errs []string
	for _, line := range strings.Split(out, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		low := strings.ToLower(t)
		if strings.Contains(low, "warning") || strings.Contains(low, "[warn]") {
			continue
		}
		// 파일:줄:칸 모양이거나 error 라는 말이 있으면 오류로 본다.
		if strings.Contains(t, ": ") && (strings.Count(t, ":") >= 2 || strings.Contains(low, "error")) {
			errs = append(errs, t)
		}
	}
	return errs
}
