package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 답을 못 받은 것은 「어느 것도 아니다」 가 아니다.
//
// 못 받은 회차를 0 으로 세면, LLM 이 안 떠 있을 때 「어느 것도 아니라고
// 3/3표로 답했다」 가 되어 기계가 이미 따라가 놓은 계약까지 버린다.
func TestSilenceIsNotRefusal(t *testing.T) {
	pick := readSource(t, "owner_pick.go")
	short := readSource(t, "shortlist.go")

	// 못 읽은 답은 표가 아니다 — 0(전혀 아니다)과 다르다.
	for _, must := range []string{
		"func (t *taskContext) askScore(prompt string) (int, bool)",
		"if n, ok := t.askScore(prompt); ok {",
		"return -1",
	} {
		if !strings.Contains(short, must) {
			t.Errorf("shortlist.go 에 %q 가 없다 — 침묵을 0점으로 센다", must)
		}
	}
	// 한 번도 못 읽었으면 거절이 아니라 미결정이다.
	if !strings.Contains(pick, "case !read:") {
		t.Error("한 번도 못 읽은 것을 거절과 갈라 다루지 않는다")
	}
	if !strings.Contains(pick, "case top <= 0:") {
		t.Error("모두 0점인 것을 거절로 다루지 않는다")
	}
}

// 선언 머리는 맨 왼쪽 선언이다. 함수 몸통에서 공장을 부르는 저장소에서
// 들여쓴 지역 변수를 집으면 이름도 뜻을 잃고 경고도 사라진다.
func TestDeclHeadIsTopLevel(t *testing.T) {
	root := fakeRepo(t, map[string]string{
		"src/protos/ceowebapis/ceoweb/v1/worker.service.ts": "export const InternalDefinition = {}\n",
		"src/grpc/ceoweb.ts": `
import { InternalDefinition } from "@/protos/ceowebapis/ceoweb/v1/worker.service";

// ⚠️ 이 서비스는 비밀번호 없이 고용주 세션을 만든다.
// 반드시 관리자 인증과 권한을 확인한 뒤에만 호출해야 한다.
export function getCEOWebWorkerClient() {
	const host = "x";
	const port = 1;
	return createClient(InternalDefinition, host, port);
}
`,
	})
	scan := repoContracts(root)
	c, ok := scan.contracts["src/protos/ceowebapis/ceoweb/v1/worker.service.ts"]
	if !ok {
		t.Fatalf("계약을 못 찾았다: %v — %s", keysOf(scan.contracts), scan.note())
	}
	if !strings.HasPrefix(c.why, "getCEOWebWorkerClient()") {
		t.Errorf("선언 머리가 지역 변수로 잡혔다: %q", c.why)
	}
	if !strings.Contains(c.note, "비밀번호 없이") {
		t.Errorf("그 함수에 달린 경고가 사라졌다: %q", c.note)
	}
}

// 빈 줄은 하나까지만 건너뛴다. 여러 줄을 건너뛰면 앞 계약의 말이 붙는다.
func TestBlankLinesStopTheComment(t *testing.T) {
	root := fakeRepo(t, map[string]string{
		"src/protos/userapis/user/v1/service.ts": "export const UserServiceDefinition = {}\n",
		"src/grpc/clients.ts": `
import { UserServiceDefinition } from "@/protos/userapis/user/v1/service";

// 앞 계약을 두고 쓴 말이다. 이 계약의 것이 아니다.


export const getUserClient = createClient(UserServiceDefinition, x);
`,
	})
	scan := repoContracts(root)
	c := scan.contracts["src/protos/userapis/user/v1/service.ts"]
	if strings.Contains(c.note, "앞 계약을 두고 쓴 말") {
		t.Errorf("빈 줄 둘을 건너뛰어 남의 말이 붙었다: %q", c.note)
	}
}

// 타입을 묻는 갈래도 원본 저장소로 보내야 한다.
func TestTypeBranchGoesToTheSource(t *testing.T) {
	src := readSource(t, "owner_of_state.go")
	if strings.Contains(src, `return owner, fmt.Sprintf("%s 는 %s 에서 온다", typeName, home)`) {
		t.Error("타입 물음 갈래가 펴낸 생성물 저장소를 그대로 돌려준다")
	}
	if strings.Count(src, "t.resolveTracedOwner(") < 3 {
		t.Error("모든 갈래가 원본으로 가지 않는다")
	}
}

var _ = filepath.Join
var _ = os.MkdirAll
