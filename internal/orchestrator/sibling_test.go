package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSiblingExample(t *testing.T) {
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
	w("tests/crud/staff.spec.ts", "import { expect, test } from '@playwright/test';\n\ntest('직원', async ({ page }) => {\n  await expect(page.locator('h1')).toContainText('직원');\n});\n")
	w("src/lib/x.ts", "export const x = 1;\n")

	got := siblingExample(dir, "tests/crud/connect.spec.ts", 30)
	if !strings.Contains(got, "async ({ page })") {
		t.Errorf("이웃이 쓰는 법을 안 보여 줬다:\n%s", got)
	}
	if !strings.Contains(got, "toContainText") {
		t.Errorf("맞는 matcher 를 안 보여 줬다:\n%s", got)
	}

	// 시험 파일이 아니면 아무것도 안 붙인다.
	if ex := siblingExample(dir, "src/lib/y.ts", 30); ex != "" {
		t.Errorf("시험 파일이 아닌데 붙였다: %q", ex)
	}
	// 자기 자신은 본보기가 아니다.
	if ex := siblingExample(dir, "tests/crud/staff.spec.ts", 30); strings.Contains(ex, "tests/crud/staff.spec.ts") {
		t.Errorf("자기 자신을 본보기로 줬다:\n%s", ex)
	}
}
