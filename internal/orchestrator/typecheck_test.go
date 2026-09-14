package orchestrator

import "testing"

const svelteOut = `
/tmp/cwchk/src/lib/server/data/connectcud.ts:129:34
Error: Property 'updateInviteStatus' does not exist on type 'InternalClient<{}>'. 
/tmp/cwchk/src/routes/(auth)/store/connect/+page.server.ts:88:17
Error: Property 'pending' does not exist on type '{ ok: true; count: number; }'. 
====================================
svelte-check found 424 errors and 1 warning in 82 files
`

func TestParseTypeErrors(t *testing.T) {
	es := parseTypeErrors(svelteOut)
	if len(es) != 2 {
		t.Fatalf("2개여야 하는데 %d개: %v", len(es), es)
	}
	if es[0].file != "connectcud.ts" {
		t.Errorf("파일 이름이 틀렸다: %q", es[0].file)
	}
}

// 손대기 전에 있던 오류는 막지 않는다. 줄이 밀려도 같은 오류로 본다.
func TestNewTypeErrorsIgnoresExisting(t *testing.T) {
	before := parseTypeErrors(`
/x/src/a.ts:10:3
Error: 원래 있던 오류. 
`)
	after := parseTypeErrors(`
/x/src/a.ts:57:3
Error: 원래 있던 오류. 
/x/src/b.ts:1:1
Error: 새로 생긴 오류. 
`)
	got := newTypeErrors(before, after)
	if len(got) != 1 || got[0].file != "b.ts" {
		t.Errorf("새 오류 하나만 나와야 한다: %v", got)
	}
}

func TestNewTypeErrorsEmptyWhenUnchanged(t *testing.T) {
	es := parseTypeErrors(svelteOut)
	if got := newTypeErrors(es, es); len(got) != 0 {
		t.Errorf("그대로면 빈 목록이어야 한다: %v", got)
	}
}

func TestTscFormat(t *testing.T) {
	es := parseTypeErrors("src/x.ts(12,5): error TS2339: Property 'q' does not exist.")
	if len(es) != 1 || es[0].file != "x.ts" {
		t.Errorf("tsc 형식을 못 읽었다: %v", es)
	}
}
