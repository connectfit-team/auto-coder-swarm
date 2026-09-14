package agent

import "strings"

// 파일 끝 줄바꿈을 지우지 않는다.
//
// 오늘 넘어온 수정 세 건이 모두 이랬다.
//
//	] as const;
//	\ No newline at end of file
//
// 고친 내용과 아무 상관이 없는데 **건드리는 파일마다** 마지막 줄바꿈이
// 사라졌다. 까닭은 쓰기 직전의 TrimSpace 다 — 모델이 붙인 군말을 걷어내려는
// 것인데 끝의 "\n" 까지 함께 걷었다.
//
// 작아 보이지만 작지 않다. 손댄 적 없는 줄이 diff 에 뜨고, 그 잡음이 매
// 파일마다 쌓여 사람이 진짜 변경을 못 찾는다. 검토자도 그 줄을 문제로 짚는다.
func ensureFinalNewline(s string) string {
	if s == "" {
		return s
	}
	if strings.HasSuffix(s, "\n") {
		return s
	}
	return s + "\n"
}
