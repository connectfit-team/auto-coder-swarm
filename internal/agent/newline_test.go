package agent

import "testing"

func TestEnsureFinalNewline(t *testing.T) {
	cases := map[string]string{
		"":                      "",
		"x":                     "x\n",
		"x\n":                   "x\n",
		"] as const;":           "] as const;\n",
		"a\nb\n":                "a\nb\n",
		"package p\n\nfunc f()": "package p\n\nfunc f()\n",
	}
	for in, want := range cases {
		if got := ensureFinalNewline(in); got != want {
			t.Errorf("%q → %q, 바람 %q", in, got, want)
		}
	}
}

// CleanCodeOutput 이 끝 줄바꿈을 먹던 것이 뿌리였다.
func TestCleanCodeOutputKeepsNewlineAfterFix(t *testing.T) {
	got := ensureFinalNewline(CleanCodeOutput("```ts\nexport const A = 1;\n```"))
	if got != "export const A = 1;\n" {
		t.Errorf("얻음 %q", got)
	}
}
