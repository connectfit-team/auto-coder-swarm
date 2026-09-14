package orchestrator

import "testing"

// 도구가 없는 것과 코드가 안 되는 것은 다르다.
// 사람이 고칠 것이 저장소인지 PATH 인지 알려 줘야 한다.
func TestMissingTool(t *testing.T) {
	cases := map[string]string{
		"bash: line 1: flutter: command not found": "flutter",
		"bash: go: command not found":              "go",
		"/bin/sh: 1: vite: not found":              "vite",
		"internal/a.go:1:1: undefined: Foo":        "",
		"":                                         "",
	}
	for in, want := range cases {
		if got := missingTool(in); got != want {
			t.Errorf("%q → %q, 기대 %q", in, got, want)
		}
	}
}

// Flutter 도 물러설 곳이 있어야 한다. 새 워크트리에는 .dart_tool 이 없다.
func TestFlutterHasLadder(t *testing.T) {
	l := ladderFor("Flutter")
	if len(l) < 2 {
		t.Fatalf("Flutter 사다리가 %d칸이다 — 물러설 곳이 없다: %v", len(l), l)
	}
	if l[0] != "flutter analyze" {
		t.Errorf("가장 엄한 것이 %q 다", l[0])
	}
}
