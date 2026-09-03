package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/connectfit-team/auto-coder-swarm/internal/insightclient"
)

func TestApplyVariantPlanInsertsBottomUp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.proto")
	os.WriteFile(path, []byte("enum E {\n  E_GOOGLE = 1;\n  E_APPLE = 2;\n  E_NAVER = 3;\n}\n"), 0o644)

	plan := insightclient.VariantRepoPlan{Repo: "r", Changes: []insightclient.VariantChange{
		{File: "r/a.proto", InsertAfter: 2, Block: []string{"  E_KAKAO = 4;"}},
		{File: "r/a.proto", InsertAfter: 4, Block: []string{"  E_INSTAGRAM = 5;"}},
	}}
	out, err := applyVariantPlan(dir, plan)
	if err != nil {
		t.Fatal(err)
	}
	if out.Inserted != 2 {
		t.Fatalf("둘을 넣어야 한다: %d", out.Inserted)
	}
	got, _ := os.ReadFile(path)
	want := "enum E {\n  E_GOOGLE = 1;\n  E_KAKAO = 4;\n  E_APPLE = 2;\n  E_NAVER = 3;\n  E_INSTAGRAM = 5;\n}\n"
	if string(got) != want {
		t.Errorf("자리가 어긋났다\n나온 것:\n%s\n바라는 것:\n%s", got, want)
	}
}

func TestApplyVariantPlanIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "b.go")
	os.WriteFile(path, []byte("var (\n\tA = \"naver\"\n)\n"), 0o644)

	plan := insightclient.VariantRepoPlan{Repo: "r", Changes: []insightclient.VariantChange{
		{File: "r/b.go", InsertAfter: 2, Block: []string{"\tB = \"instagram\""}},
	}}
	for i := 0; i < 3; i++ {
		out, err := applyVariantPlan(dir, plan)
		if err != nil {
			t.Fatal(err)
		}
		if i > 0 && out.Inserted != 0 {
			t.Errorf("%d번째에 또 넣었다", i+1)
		}
	}
	got, _ := os.ReadFile(path)
	if n := strings.Count(string(got), "instagram"); n != 1 {
		t.Errorf("세 번 돌렸는데 %d번 들어갔다", n)
	}
}

func TestApplyVariantPlanRefusesOutOfRange(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "c.go"), []byte("package x\n"), 0o644)
	_, err := applyVariantPlan(dir, insightclient.VariantRepoPlan{
		Repo: "r", Changes: []insightclient.VariantChange{
			{File: "r/c.go", InsertAfter: 999, Block: []string{"// x"}},
		}})
	if err == nil {
		t.Error("파일 범위 밖인데 조용히 넘어갔다")
	}
}
