package orchestrator

import "testing"

// 1회차가 **사본을 만들기 전에** 돌아가면 2회차가 만들어야 한다.
//
// 짚어 준 자리를 계획이 빠뜨리면(#173) 그 검사가 사본 만드는 블록보다 먼저
// 반환한다. `attempt == 1` 만 보면 그 뒤로 영영 안 만들어, 빈 경로에 대고
// 쓰다가 "폴더가 없다" 로 끝난다(W-62666, 수정 0바이트).
func TestWorkspaceSetupSurvivesEarlyReturn(t *testing.T) {
	tc := &taskContext{}
	if !tc.needsWorkspaceSetup(1) {
		t.Fatal("1회차에 사본을 안 만든다")
	}
	if !tc.needsWorkspaceSetup(2) {
		t.Fatal("1회차가 만들기 전에 돌아갔는데 2회차도 안 만든다 — W-62666 이 그래서 0바이트였다")
	}
	tc.repoPath = "/tmp/swarm_ws_x/repo"
	if tc.needsWorkspaceSetup(2) {
		t.Fatal("이미 만든 사본을 또 만든다")
	}
	if !tc.needsWorkspaceSetup(1) {
		t.Fatal("1회차는 언제나 새로 만든다 — 앞 작업의 사본을 물려받으면 안 된다")
	}
}

func TestBranchSafeKeepsTaskIDApart(t *testing.T) {
	if got := branchSafe("W-62666"); got != "W-62666" {
		t.Fatalf("작업 번호가 망가졌다: %q", got)
	}
	// 가지 이름에 못 쓰는 글자는 턴다.
	if got := branchSafe("W/62 666~^:?*[\\"); got != "W62666" {
		t.Fatalf("못 쓰는 글자가 남았다: %q", got)
	}
	if branchSafe("W-1") == branchSafe("W-2") {
		t.Fatal("다른 작업이 같은 가지 이름을 받는다")
	}
}
