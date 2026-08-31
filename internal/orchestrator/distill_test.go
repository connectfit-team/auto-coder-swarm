package orchestrator

import (
	"strings"
	"testing"
)

func TestDistillBuildError(t *testing.T) {
	// 실제로 겪은 모양: 손대지도 않은 파일의 A11y 경고가 앞을 다 차지하고
	// 진짜 오류는 맨 뒤에 있다.
	out := `> cms@0.0.1 build
> vite build

vite v4.5.14 building SSR bundle for production...
transforming...
6:22:19 PM [vite-plugin-svelte] src/routes/admin/logs/+page.svelte:84:16 A11y: A form label must be associated with a control.
6:22:19 PM [vite-plugin-svelte] src/routes/admin/logs/+page.svelte:94:16 A11y: A form label must be associated with a control.
src/lib/server/db/attendance.ts:42:9 - error TS2304: Cannot find name 'endOfMonth'.
  42   const end = endOfMonth(month);
             ~~~~~~~~~~`

	got := distillBuildError(out)
	if !strings.Contains(got, "TS2304") {
		t.Errorf("진짜 오류가 빠졌다:\n%s", got)
	}
	if strings.Contains(got, "A11y") {
		t.Errorf("경고를 걷어내지 못했다:\n%s", got)
	}
	if !strings.Contains(got, "endOfMonth(month)") {
		t.Errorf("오류 다음 줄의 위치 정보가 빠졌다:\n%s", got)
	}
}

func TestDistillBuildErrorFallsBackToTail(t *testing.T) {
	// 아무 표시도 없으면 앞이 아니라 뒤를 준다.
	var b strings.Builder
	for i := 0; i < 100; i++ {
		b.WriteString("noise line\n")
	}
	b.WriteString("마지막 줄")
	got := distillBuildError(b.String())
	if !strings.Contains(got, "마지막 줄") {
		t.Errorf("끝부분을 주지 않았다:\n%s", got)
	}
}
