package agent

import (
	"strings"
	"testing"
)

func TestApplyEditBlocksToleratesIndent(t *testing.T) {
	// 모델은 자리를 정확히 짚고도 앞 공백을 다르게 적는다. 실제로 그래서
	// "원문에 없는 내용" 으로 버려졌다.
	orig := "export const f = async () => {\n        workAt: {\n            lt: new Date(y, m + 1, 0)\n        }\n};\n"
	raw := "<<<<<<< SEARCH\nlt: new Date(y, m + 1, 0)\n=======\nlt: new Date(y, m + 1, 1)\n>>>>>>> REPLACE"
	got, err := applyEditBlocks(orig, raw)
	if err != nil {
		t.Fatalf("들여쓰기가 달라 못 찾았다: %v", err)
	}
	if !strings.Contains(got, "m + 1, 1") {
		t.Errorf("바뀌지 않았다:\n%s", got)
	}
	if !strings.Contains(got, "export const f") {
		t.Errorf("나머지가 사라졌다:\n%s", got)
	}
}

func TestApplyEditBlocksLooseStillRejectsAmbiguous(t *testing.T) {
	orig := "  x();\n    x();\n"
	raw := "<<<<<<< SEARCH\nx();\n=======\ny();\n>>>>>>> REPLACE"
	if _, err := applyEditBlocks(orig, raw); err == nil {
		t.Error("공백을 무시하니 두 군데인데 그냥 바꿨다")
	}
}

func TestRelevantRegionsNarrowsInsteadOfGivingUp(t *testing.T) {
	// 모든 줄에 이름이 있으면 예전에는 포기하고 통째로 넘겨 창을 넘겼다.
	var b strings.Builder
	for i := 0; i < 400; i++ {
		b.WriteString("const value = 1;\n")
	}
	got := relevantRegions(b.String(), "value 를 고쳐라")
	// 좁혀도 400줄 전부가 걸리므로 결국 빈 문자열이지만, 그때는 부르는 쪽이
	// 글자 수로 자른다. 여기서는 통째 원문을 그대로 돌려주지 않는 것만 본다.
	if got != "" && strings.Count(got, "\n") > regionMaxLines {
		t.Errorf("여전히 너무 많이 준다: %d줄", strings.Count(got, "\n"))
	}
}
