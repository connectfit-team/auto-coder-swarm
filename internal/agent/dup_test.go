package agent

import (
	"strings"
	"testing"
)

func TestApplyEditBlocksFixesEveryCopy(t *testing.T) {
	// workplace.ts 의 진짜 모양 — 같은 말일 경계 버그가 두 곳에 복사돼 있다.
	orig := `export const getA = async () => {
    return db.findMany({
        workAt: {
            lt: new Date(new Date().getFullYear(), new Date().getMonth() + 1, 0)
        }
    });
};

export const getB = async () => {
    return db.findMany({
        workAt: {
            lt: new Date(new Date().getFullYear(), new Date().getMonth() + 1, 0)
        }
    });
};
`
	raw := "<<<<<<< SEARCH\nlt: new Date(new Date().getFullYear(), new Date().getMonth() + 1, 0)\n" +
		"=======\nlt: new Date(new Date().getFullYear(), new Date().getMonth() + 1, 1)\n>>>>>>> REPLACE"

	got, err := applyEditBlocks(orig, raw)
	if err != nil {
		t.Fatalf("복사된 두 곳을 못 고쳤다: %v", err)
	}
	if n := strings.Count(got, "getMonth() + 1, 1)"); n != 2 {
		t.Errorf("고쳐진 자리 %d개, want 2:\n%s", n, got)
	}
	if strings.Contains(got, "getMonth() + 1, 0)") {
		t.Errorf("안 고쳐진 자리가 남았다:\n%s", got)
	}
	if !strings.Contains(got, "export const getA") || !strings.Contains(got, "export const getB") {
		t.Errorf("나머지가 사라졌다:\n%s", got)
	}
	// 들여쓰기를 물려받아야 한다.
	if !strings.Contains(got, "            lt: new Date") {
		t.Errorf("들여쓰기가 무너졌다:\n%s", got)
	}
}

func TestApplyEditBlocksStillRefusesShortAmbiguous(t *testing.T) {
	// 짧은 조각이 여러 군데 맞으면 바꾸지 않는다 — 파일이 망가진다.
	orig := "a {\n  x\n}\nb {\n  x\n}\n"
	raw := "<<<<<<< SEARCH\nx\n=======\ny\n>>>>>>> REPLACE"
	if _, err := applyEditBlocks(orig, raw); err == nil {
		t.Error("짧고 모호한 내용을 전부 바꿔 버렸다")
	}
}

func TestIsSubstantial(t *testing.T) {
	if isSubstantial("}") || isSubstantial("  x  ") {
		t.Error("짧은 조각을 충분하다고 봤다")
	}
	if !isSubstantial("lt: new Date(y, m + 1, 0)") {
		t.Error("자리를 특정할 만한 조각을 짧다고 봤다")
	}
}
