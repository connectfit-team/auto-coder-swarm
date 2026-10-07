package orchestrator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// 코더가 같은 이름을 두 파일에 두 번 만드는 것을 잡는다.
//
// 실측 W-96045 는 `message RequestUpdateRequest` 를, W-11906 은
// `enum HoldStatus` 를 communication·service 양쪽에 만들었다. protoc 이면
// 중복 정의로 깨지는데 `go build` 는 통과한다.
func TestDeclsAddedThisTurnSeesBothFiles(t *testing.T) {
	repo := protoRepoForTest(t, "syntax = \"proto3\";\npackage ceoweb.v1;\n\nmessage Keep {\n  int32 status = 1;\n}\n")
	// 두 번째 파일을 만들어 커밋한다 — 둘 다 「있던 파일」이어야 한다.
	second := filepath.Join(repo, "ceoweb", "v1", "y.proto")
	write(t, second, "syntax = \"proto3\";\npackage ceoweb.v1;\n\nmessage Other {\n  int32 state = 1;\n}\n")
	gitRun(t, repo, "add", ".")
	gitRun(t, repo, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "-m", "second")

	// 코더가 같은 이름을 양쪽에 더했다.
	appendTo(t, filepath.Join(repo, "ceoweb", "v1", "x.proto"), "\nmessage RequestUpdateRequest {\n  int32 pending_status = 1;\n}\n")
	appendTo(t, second, "\nmessage RequestUpdateRequest {\n  int32 pending_status = 1;\n}\nenum HoldStatus {\n  HOLD_STATUS_UNSPECIFIED = 0;\n}\n")

	added := declsAddedThisTurn(repo)
	for _, want := range []string{"RequestUpdateRequest", "HoldStatus"} {
		if _, ok := added[want]; !ok {
			t.Errorf("%s 를 못 봤다: %v", want, added)
		}
	}
	if _, ok := added["Keep"]; ok {
		t.Error("있던 선언을 새로 더한 것으로 봤다")
	}
}

func TestAlreadyAddedNoteNamesThemAndWarns(t *testing.T) {
	note := alreadyAddedNote(map[string]string{
		"RequestUpdateRequest": "ceoweb/v1/connect.communication.proto",
		"HoldStatus":           "ceoweb/v1/connect.service.proto",
	}, "ceoweb/v1/connect.service.proto")
	for _, want := range []string{
		"RequestUpdateRequest", "HoldStatus",
		"connect.communication.proto", "다시 선언하지 마라", "지금 고치는 파일",
	} {
		if !strings.Contains(note, want) {
			t.Errorf("%q 가 없다:\n%s", want, note)
		}
	}
}

// 더한 것이 없으면 아무 말도 붙이지 않는다 — 빈 쪽지는 창만 먹는다.
func TestAlreadyAddedNoteSaysNothingWhenNothingAdded(t *testing.T) {
	if got := alreadyAddedNote(nil, "x.proto"); got != "" {
		t.Fatalf("빈 쪽지를 냈다: %q", got)
	}
	if got := alreadyAddedNote(map[string]string{}, "x.proto"); got != "" {
		t.Fatalf("빈 쪽지를 냈다: %q", got)
	}
}

// 사본이 없거나 git 이 아니어도 죽지 않는다.
func TestDeclsAddedThisTurnIsSafeWithoutRepo(t *testing.T) {
	if got := declsAddedThisTurn(""); len(got) != 0 {
		t.Fatal("빈 경로에서 무언가 읽었다")
	}
	if got := declsAddedThisTurn(t.TempDir()); len(got) != 0 {
		t.Fatal("git 이 아닌 곳에서 무언가 읽었다")
	}
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func appendTo(t *testing.T, path, body string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	write(t, path, string(b)+body)
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
}
