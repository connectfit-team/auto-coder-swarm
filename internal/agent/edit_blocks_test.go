package agent

import (
	"strings"
	"testing"
)

func TestApplyEditBlocks(t *testing.T) {
	orig := "const a = 1;\nconst end = new Date(y, m + 1, 0);\nconst b = 2;\n"
	raw := "<<<<<<< SEARCH\nconst end = new Date(y, m + 1, 0);\n=======\nconst end = new Date(y, m + 1, 1);\n>>>>>>> REPLACE"
	got, err := applyEditBlocks(orig, raw)
	if err != nil {
		t.Fatalf("적용 실패: %v", err)
	}
	if !strings.Contains(got, "m + 1, 1") || !strings.Contains(got, "const b = 2;") {
		t.Errorf("잘못 적용됐다:\n%s", got)
	}
}

func TestApplyEditBlocksRejectsAmbiguous(t *testing.T) {
	// 같은 줄이 두 번 있으면 어디를 고치는지 알 수 없다.
	orig := "x();\nx();\n"
	raw := "<<<<<<< SEARCH\nx();\n=======\ny();\n>>>>>>> REPLACE"
	if _, err := applyEditBlocks(orig, raw); err == nil {
		t.Error("여러 번 나오는 내용을 그냥 바꿨다")
	}
}

func TestApplyEditBlocksRejectsMissing(t *testing.T) {
	if _, err := applyEditBlocks("a\n", "<<<<<<< SEARCH\nzzz\n=======\nb\n>>>>>>> REPLACE"); err == nil {
		t.Error("원문에 없는 내용을 찾았다고 했다")
	}
	if _, err := applyEditBlocks("a\n", "그냥 설명만 씀"); err == nil {
		t.Error("블록이 없는데 통과했다")
	}
}

func TestLostExports(t *testing.T) {
	before := `export function getAttendanceRecords() {}
export const LIMIT = 10;
export class Foo {}
function helper() {}
`
	// 모델이 통째로 다시 쓰다 함수 하나를 빠뜨린 모양.
	after := `export const LIMIT = 10;
export class Foo {}
function helper() {}
`
	lost := lostExports(before, after)
	if len(lost) != 1 || lost[0] != "getAttendanceRecords" {
		t.Errorf("사라진 export = %v, want [getAttendanceRecords]", lost)
	}
	if l := lostExports(before, before); len(l) != 0 {
		t.Errorf("안 바뀐 파일에서 사라졌다고 했다: %v", l)
	}
}

func TestExportedNamesGo(t *testing.T) {
	names := exportedNames("func GetWorkplace() {}\nfunc helper() {}\n")
	if !names["GetWorkplace"] || names["helper"] {
		t.Errorf("Go 대문자 함수 판정이 틀렸다: %v", names)
	}
}
