package orchestrator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
)

func healRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, p := range []string{
		"src/routes/(auth)/store/connect/+page.server.ts",
		"src/routes/(auth)/more/+page.server.ts",
		"src/lib/server/data/connectcud.ts",
	} {
		full := filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestResolveHealTarget(t *testing.T) {
	dir := healRepo(t)
	plan := planOf("src/routes/(auth)/store/connect/+page.server.ts", "src/lib/server/data/connectcud.ts")

	// 짧게 적어도 계획에 하나뿐이면 찾아 준다.
	got, ok := resolveHealTarget(dir, "+page.server.ts", plan, "")
	if !ok || got != "src/routes/(auth)/store/connect/+page.server.ts" {
		t.Errorf("못 찾았다: %q %v", got, ok)
	}

	// 온전한 경로는 그대로 쓴다.
	got, ok = resolveHealTarget(dir, "src/lib/server/data/connectcud.ts", plan, "")
	if !ok || got != "src/lib/server/data/connectcud.ts" {
		t.Errorf("온전한 경로를 바꿨다: %q %v", got, ok)
	}

	// 둘로 좁혀지면 고르지 않는다 — 어느 쪽인지 알 수 없다.
	two := planOf("src/routes/(auth)/store/connect/+page.server.ts", "src/routes/(auth)/more/+page.server.ts")
	if _, ok := resolveHealTarget(dir, "+page.server.ts", two, ""); ok {
		t.Error("둘 가운데 하나를 골랐다")
	}

	// 계획에 없어도 빌드 오류에 나왔으면 찾는다.
	fail := "error during build:\nsrc/lib/server/data/connectcud.ts(12,5): error TS2339: x"
	got, ok = resolveHealTarget(dir, "connectcud.ts", agent.Plan{}, fail)
	if !ok || got != "src/lib/server/data/connectcud.ts" {
		t.Errorf("오류에 나온 경로를 못 찾았다: %q %v", got, ok)
	}

	// 아예 없는 파일은 못 찾는다.
	if _, ok := resolveHealTarget(dir, "nope.ts", plan, ""); ok {
		t.Error("없는 파일을 찾았다고 했다")
	}
}
