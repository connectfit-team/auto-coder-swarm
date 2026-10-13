package orchestrator

import (
	"context"
	"strings"
	"testing"
)

// 저장소 바깥은 못 읽는다 — 도구가 기계를 뒤지는 통로가 되면 안 된다.
func TestReadFileStaysInsideTheRepo(t *testing.T) {
	repo := gitRepoWith(t, map[string]string{"a/x.proto": "message A {}\n"})

	got, err := readRepoFile(repo, "a/x.proto")
	if err != nil || !strings.Contains(got, "message A") {
		t.Fatalf("안쪽 파일을 못 읽는다: %v %q", err, got)
	}
	for _, bad := range []string{"../../etc/passwd", "/etc/passwd", "a/../../../etc/passwd"} {
		if _, err := readRepoFile(repo, bad); err == nil {
			t.Errorf("바깥을 읽었다: %s", bad)
		}
	}
	if _, err := readRepoFile("", "a/x.proto"); err == nil {
		t.Error("저장소가 없는데 읽었다")
	}
}

func TestSearchRepo(t *testing.T) {
	repo := gitRepoWith(t, map[string]string{
		"a/x.proto": "message ConnectInfo {\n  string id = 1;\n}\n",
		"b/y.ts":    "const ConnectInfo = 1;\n",
	})
	ctx := context.Background()

	got, err := searchRepo(ctx, repo, "ConnectInfo", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "a/x.proto") || !strings.Contains(got, "b/y.ts") {
		t.Errorf("둘 다 찾아야 한다:\n%s", got)
	}

	// 갈래를 좁힐 수 있어야 한다.
	got, _ = searchRepo(ctx, repo, "ConnectInfo", "*.proto")
	if strings.Contains(got, "b/y.ts") {
		t.Errorf("좁혔는데 다른 것이 나온다:\n%s", got)
	}

	// 못 찾은 것은 오류가 아니다 — git grep 은 1 로 끝난다.
	got, err = searchRepo(ctx, repo, "NoSuchName", "")
	if err != nil {
		t.Errorf("못 찾은 것을 오류로 냈다: %v", err)
	}
	if !strings.Contains(got, "없다") {
		t.Errorf("없다고 말하지 않는다: %q", got)
	}
}

// 도구가 알리는 판정은 handOver 와 **같은 함수**여야 한다.
// 두 곳에서 따로 판정하면 도구가 통과라 한 것을 관문이 막는다.
func TestCheckToolUsesTheSameGates(t *testing.T) {
	src := readSource(t, "codertools.go")
	if !strings.Contains(src, "t.gateChecks(diff)") {
		t.Error("check 도구가 gateChecks 를 안 쓴다")
	}
	if !strings.Contains(src, "t.currentDiff()") {
		t.Error("check 도구가 지금 작업 트리를 안 본다")
	}
	// 관문을 따로 부르면 어긋난다.
	for _, forbidden := range []string{"CheckProtoChange(", "CheckToolLeak(", "fieldWentIntoNewMessage("} {
		if strings.Contains(src, forbidden) {
			t.Errorf("관문을 따로 부른다: %s — gateChecks 하나만 써라", forbidden)
		}
	}
}

// **어느 파일을 고쳤는지 적어 준다.** 안 적으면 모델이 경로를 지어낸다 —
// 실측에서 `path/to/your/file.proto` 를 불러 판마다 첫 턴을 버렸다.
func TestGateReportNamesTheTouchedFiles(t *testing.T) {
	diff := "--- a/proto/connect.proto\n+++ b/proto/connect.proto\n@@\n+  bool held = 2;\n" +
		"--- a/proto/service.proto\n+++ b/proto/service.proto\n@@\n+message Dup {}\n"

	got := GateReport(diff, []string{"## PROTO_BROKEN\n뭔가 틀렸다"})
	for _, want := range []string{"proto/connect.proto", "proto/service.proto", "지어내지 말고"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q 가 없다:\n%s", want, got)
		}
	}
	if !strings.Contains(got, "뭔가 틀렸다") {
		t.Error("관문 내용을 빠뜨렸다")
	}

	// 지나면 군더더기를 안 붙인다.
	if got := GateReport(diff, nil); got != "관문을 지난다." {
		t.Errorf("지나는데 말이 많다: %q", got)
	}
}
