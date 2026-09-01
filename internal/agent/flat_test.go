package agent

import (
	"strings"
	"testing"
)

func TestApplyEditBlocksToleratesReflow(t *testing.T) {
	// 분석이 코드를 한 줄로 펴서 적어 준다. 원문은 네 줄이다.
	orig := `export const getWorkplaces = async () => {
    const rows = await db.findMany({
        workAt: {
            gte: new Date(new Date().getFullYear(), new Date().getMonth(), 1),
            lt: new Date(new Date().getFullYear(), new Date().getMonth() + 1, 0)
        }
    });
};
`
	raw := "<<<<<<< SEARCH\nworkAt: { gte: new Date(new Date().getFullYear(), new Date().getMonth(), 1), lt: new Date(new Date().getFullYear(), new Date().getMonth() + 1, 0) }\n" +
		"=======\nworkAt: { gte: new Date(new Date().getFullYear(), new Date().getMonth(), 1), lt: new Date(new Date().getFullYear(), new Date().getMonth() + 1, 1) }\n>>>>>>> REPLACE"

	got, err := applyEditBlocks(orig, raw)
	if err != nil {
		t.Fatalf("줄바꿈이 달라 못 찾았다: %v", err)
	}
	if !strings.Contains(got, "getMonth() + 1, 1)") {
		t.Errorf("말일 경계가 안 고쳐졌다:\n%s", got)
	}
	if !strings.Contains(got, "export const getWorkplaces") || !strings.Contains(got, "db.findMany") {
		t.Errorf("나머지가 사라졌다:\n%s", got)
	}
}

func TestReplaceFlattenedRejectsAmbiguous(t *testing.T) {
	src := []string{"a(", "  1", ")", "a(", "  1", ")"}
	if _, err := replaceFlattened(src, "a( 1 )", "b()"); err == nil {
		t.Error("두 군데인데 그냥 바꿨다")
	}
}
