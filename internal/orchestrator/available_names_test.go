package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixtureRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write := func(p, s string) {
		full := filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("src/lib/server/grpc/clients.ts", `
export const getConnectClient = createGuardedClient<A, B>(x);
export const getSalaryClient = createGuardedClient<C, D>(y);
`)
	write("src/lib/types/session.ts", "export type Session = { id: string };\nexport function initSession() {}\n")
	write("src/lib/server/data/connectcud.ts", `
import { getConnectClient } from "$lib/server/grpc/clients";
import type { Session } from "$lib/types/session";
export async function listSent() {
    return getConnectClient().listSentInvites({});
}
`)
	// 같은 공장이 다른 파일에서도 쓰인다 — 저장소 전체를 훑어야 다 모인다.
	write("src/routes/connect/+page.server.ts", `
import { getConnectClient } from "$lib/server/grpc/clients";
export const actions = {
    a: () => getConnectClient().acceptRequest({}),
    b: () => getConnectClient().rejectRequest({}),
};
`)
	return dir
}

func TestAvailableNames(t *testing.T) {
	dir := fixtureRepo(t)
	got := AvailableNames(dir, "src/lib/server/data/connectcud.ts")

	for _, want := range []string{
		"getConnectClient", "getSalaryClient", // 모듈이 내보낸 것
		"Session", "initSession",
		"listSentInvites", "acceptRequest", "rejectRequest", // 저장소가 실제로 부르는 RPC
	} {
		if !strings.Contains(got, want) {
			t.Errorf("%q 가 쪽지에 없다:\n%s", want, got)
		}
	}
	// 지어낸 이름은 당연히 없어야 한다 — 그것이 이 쪽지의 쓸모다.
	if strings.Contains(got, "updateInviteStatus") {
		t.Error("없는 이름이 쪽지에 들어갔다")
	}
}

// 저장소 밖 모듈만 들여오는 파일에는 붙일 것이 없다.
func TestAvailableNamesEmptyForExternalOnly(t *testing.T) {
	dir := fixtureRepo(t)
	p := filepath.Join(dir, "src/lib/only-external.ts")
	if err := os.WriteFile(p, []byte(`import { z } from "zod";`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := AvailableNames(dir, "src/lib/only-external.ts"); got != "" {
		t.Errorf("붙일 것이 없어야 한다: %q", got)
	}
}

// Svelte 컴포넌트의 `export let` 은 속성이지 가져올 이름이 아니다.
//
// 이 쪽지가 그것을 "있는 이름" 으로 적어서 코더가 그대로 가져왔고, 빌드가
// `Identifier 'description' has already been declared` 로 깨졌다(W-66081).
func TestAvailableNamesSvelteProps(t *testing.T) {
	dir := t.TempDir()
	w := func(p, s string) {
		full := filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	w("src/lib/components/web/PageHeader.svelte",
		"<script lang=\"ts\">\n    export let title: string;\n    export let description = '';\n</script>\n")
	w("src/routes/x/+page.svelte",
		"<script lang=\"ts\">\n    import PageHeader from '$lib/components/web/PageHeader.svelte';\n</script>\n")

	got := AvailableNames(dir, "src/routes/x/+page.svelte")
	if !strings.Contains(got, "기본 내보내기 PageHeader") {
		t.Errorf("기본 내보내기를 안 알려 줬다:\n%s", got)
	}
	if !strings.Contains(got, "가져오는 이름이 아니다") {
		t.Errorf("속성과 가져올 이름을 안 갈랐다:\n%s", got)
	}
	// 속성이 "→" 뒤의 가져올 이름 자리에 오면 안 된다.
	for _, line := range strings.Split(got, "\n") {
		if strings.Contains(line, "PageHeader.svelte →") && strings.Contains(line, "description") {
			t.Errorf("속성을 가져올 이름처럼 적었다: %s", line)
		}
	}
}
