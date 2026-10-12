package orchestrator

import (
	"os"
	"strings"
	"testing"
)

// 임자 저장소의 메시지 이름만 모은다.
//
// 웹 저장소의 타입 이름을 보고 고르게 하면 거기만의 것을 고른다 — 실측
// W-20458 이 `InviteCandidate`(gig_ceo_web 의 TypeScript 타입)를 3표/3번으로
// 골랐다. 계약에 없는 이름이라 자식이 찾지 못하고 `ReceivedRequest` 를 service
// 파일에 다시 선언했다(W-34815, 판정 2b 불합격).
func TestOwnerMessageNamesAreFromTheContract(t *testing.T) {
	repo := "/home/cnf/cie-repos/proto-ceowebapis"
	if _, err := os.Stat(repo); err != nil {
		t.Skip("계약 사본이 없다")
	}
	names := ownerMessageNames(repo)
	if len(names) == 0 {
		t.Fatal("메시지 이름을 하나도 못 모았다")
	}
	has := map[string]bool{}
	for _, n := range names {
		has[n] = true
	}
	if !has["ReceivedRequest"] {
		t.Error("정답인 ReceivedRequest 가 목록에 없다")
	}
	// 웹에만 있는 타입은 들어오면 안 된다.
	if has["InviteCandidate"] {
		t.Error("계약에 없는 InviteCandidate 가 목록에 들어왔다")
	}
	if len(names) > ownerNameCap {
		t.Errorf("상한을 넘겼다: %d개 (상한 %d)", len(names), ownerNameCap)
	}
}

// 차례가 회차마다 같아야 두 판을 견줄 수 있다.
func TestOwnerMessageNamesAreStable(t *testing.T) {
	repo := "/home/cnf/cie-repos/proto-ceowebapis"
	if _, err := os.Stat(repo); err != nil {
		t.Skip("계약 사본이 없다")
	}
	a := strings.Join(ownerMessageNames(repo), ",")
	for i := 0; i < 3; i++ {
		if b := strings.Join(ownerMessageNames(repo), ","); b != a {
			t.Fatal("회차마다 차례가 다르다")
		}
	}
}

// 사본이 없으면 빈 목록 — 죽지 않는다.
func TestOwnerMessageNamesWithoutRepo(t *testing.T) {
	if got := ownerMessageNames(""); len(got) != 0 {
		t.Fatalf("빈 경로에서 무언가 읽었다: %v", got)
	}
	if got := ownerMessageNames(t.TempDir()); len(got) != 0 {
		t.Fatalf("빈 폴더에서 무언가 읽었다: %v", got)
	}
}

// 계약에 있는 이름인지 보는 검사가 실제로 가른다.
func TestDeclaringFileTellsContractFromWeb(t *testing.T) {
	repo := "/home/cnf/cie-repos/proto-ceowebapis"
	if _, err := os.Stat(repo); err != nil {
		t.Skip("계약 사본이 없다")
	}
	if declaringFileAtHead(repo, "ReceivedRequest") == "" {
		t.Error("계약에 있는 메시지를 없다고 한다")
	}
	if declaringFileAtHead(repo, "InviteCandidate") != "" {
		t.Error("계약에 없는 이름을 있다고 한다")
	}
}
