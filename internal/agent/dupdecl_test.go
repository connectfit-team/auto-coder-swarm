package agent

import (
	"strings"
	"testing"
)

func TestDuplicateDecls(t *testing.T) {
	// W-71948 — 같은 이름을 두 번 선언해 빌드가 깨졌다.
	bad := `<script lang="ts">
    let pending = false;
    $: rows = [];
    let pending = true;
</script>
<div></div>
`
	if dup := duplicateDecls("+page.svelte", bad); len(dup) == 0 {
		t.Error("두 번 선언했는데 못 잡았다")
	} else if !strings.Contains(strings.Join(dup, ","), "pending") {
		t.Errorf("이름이 틀렸다: %v", dup)
	}

	// 함수 안쪽의 같은 이름은 서로 다른 유효범위다 — 막으면 안 된다.
	ok := `<script lang="ts">
    function a() {
        const x = 1;
    }
    function b() {
        const x = 2;
    }
</script>
`
	if dup := duplicateDecls("+page.svelte", ok); len(dup) != 0 {
		t.Errorf("유효범위가 다른데 막았다: %v", dup)
	}

	// 마크업의 같은 낱말은 선언이 아니다.
	markup := "<script lang=\"ts\">\n    let title = '';\n</script>\n<div>let title = 'x'</div>\n"
	if dup := duplicateDecls("+page.svelte", markup); len(dup) != 0 {
		t.Errorf("마크업을 선언으로 셌다: %v", dup)
	}

	// Go
	goDup := "package p\n\nfunc F() {}\n\nfunc F() {}\n"
	if dup := duplicateDecls("x.go", goDup); len(dup) == 0 {
		t.Error("Go 중복 선언을 못 잡았다")
	}
	goOK := "package p\n\nfunc F() {\n\tx := 1\n\t_ = x\n}\n\nfunc G() {\n\tx := 2\n\t_ = x\n}\n"
	if dup := duplicateDecls("x.go", goOK); len(dup) != 0 {
		t.Errorf("멀쩡한 Go 를 막았다: %v", dup)
	}
}
