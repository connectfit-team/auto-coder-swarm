package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 「원래 있던 것」 은 HEAD 다 — 디스크의 지금 내용이 아니다.
//
// 되돌리기를 없앤 뒤(#182) 앞 회차가 쓴 것이 파일에 남는다. 그것을 기준으로
// 삼으면 **앞 회차의 잘못을 고치는 것**이 「있던 이름이 사라졌다」 가 된다.
// 실측 W-42846 이 그랬다 — #197 이 접두형으로 고치라 해서 이름을 바꿨더니
// `lostExports` 가 막아 세 시도를 다 썼다. 관문끼리 부딪힌 것이다.
func TestHeadBaselineIgnoresThisTurnsWork(t *testing.T) {
	repo := protoRepoForTest(t, "syntax = \"proto3\";\npackage x.v1;\n\nmessage Keep {\n  int32 status = 1;\n}\n")
	full := filepath.Join(repo, "ceoweb", "v1", "x.proto")

	// 앞 회차가 접미형 이름을 만들어 두었다 (아직 커밋 안 됨).
	appendTo(t, full, "\nmessage UpdateHoldStatusRequest {\n  int32 status = 1;\n}\n")

	base := headBaseline(repo)
	if base == nil {
		t.Fatal("기준을 못 만들었다")
	}
	got, ok := base(full)
	if !ok {
		t.Fatal("HEAD 에 있는 파일을 못 읽었다")
	}
	if strings.Contains(got, "UpdateHoldStatusRequest") {
		t.Fatal("앞 회차가 쓴 것이 기준에 섞였다 — 그걸 고치면 「잃었다」 가 된다")
	}
	if !strings.Contains(got, "message Keep") {
		t.Fatal("HEAD 에 있던 것이 기준에서 빠졌다")
	}
}

// HEAD 에 없는 파일(이번에 새로 만드는 것)은 못 찾았다고 답한다.
// 그때는 부르는 쪽이 디스크 내용을 기준으로 삼는다 — 전과 같다.
func TestHeadBaselineSaysNoForNewFiles(t *testing.T) {
	repo := protoRepoForTest(t, "syntax = \"proto3\";\npackage x.v1;\n\nmessage Keep {\n  int32 status = 1;\n}\n")
	base := headBaseline(repo)
	newFile := filepath.Join(repo, "ceoweb", "v1", "brand_new.proto")
	os.WriteFile(newFile, []byte("syntax = \"proto3\";\n"), 0o644)
	if _, ok := base(newFile); ok {
		t.Fatal("HEAD 에 없는 파일을 있다고 했다")
	}
}

// 사본 밖의 경로는 읽지 않는다.
func TestHeadBaselineRefusesOutsidePaths(t *testing.T) {
	repo := protoRepoForTest(t, "syntax = \"proto3\";\npackage x.v1;\n\nmessage Keep {\n  int32 status = 1;\n}\n")
	base := headBaseline(repo)
	if _, ok := base("/etc/passwd"); ok {
		t.Fatal("사본 밖을 읽었다")
	}
	if headBaseline("") != nil {
		t.Fatal("빈 경로에 기준을 만들었다")
	}
}
