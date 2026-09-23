package orchestrator

import (
	"strings"
	"testing"
)

// 계약 저장소에서 대상 파일을 「본뜰 이웃」 으로 내밀면 그 옆에 같은 것을
// 하나 더 만든다 — 실측으로 있는 service 옆에 새 service 가 생겼다.
func TestContractTaskEditsInPlace(t *testing.T) {
	b := NewFeatureBrief("proto-ceowebapis 저장소에 이것을 더해라: 보류 상태",
		"", []string{"ceoweb/v1/connect.communication.proto"}, true)

	if strings.Contains(b, "본뜰 만한 이웃 코드") {
		t.Fatalf("고칠 파일을 본뜰 이웃으로 내민다:\n%s", b)
	}
	for _, want := range []string{"고칠 파일 — 이 안에서 고른다", "그 파일 자체를 고친다", "새 파일을 만들지 않는다"} {
		if !strings.Contains(b, want) {
			t.Fatalf("%q 가 없다:\n%s", want, b)
		}
	}
	if strings.Contains(b, "새 파일을 만들어도 된다") {
		t.Fatalf("새 파일을 열어 준다:\n%s", b)
	}
}

// 고칠 파일을 못 짚었으면 본떠 만드는 수밖에 없다.
func TestContractTaskWithoutCandidatesStillBuilds(t *testing.T) {
	b := NewFeatureBrief("proto-ceowebapis 저장소에 이것을 더해라", "", nil, true)
	if !strings.Contains(b, "새 파일을 만들어도 된다") {
		t.Fatalf("고칠 파일도 없고 새로 만들 길도 없다:\n%s", b)
	}
	// 그래도 제 일을 넘기지는 않는다.
	if strings.Contains(b, "4. proto 메시지·필드가 필요하면") {
		t.Fatalf("제 일을 넘기라고 시킨다:\n%s", b)
	}
}

// 소비하는 저장소는 본떠 만드는 것이 맞다.
func TestConsumerTaskStillImitates(t *testing.T) {
	b := NewFeatureBrief("고용주웹에서 연결보류 기능을 추가할거야", "",
		[]string{"src/lib/server/connect.ts"}, false)
	if !strings.Contains(b, "본뜰 만한 이웃 코드") {
		t.Fatalf("본뜰 이웃을 안 준다:\n%s", b)
	}
	if strings.Contains(b, "새 파일을 만들지 않는다") {
		t.Fatalf("소비하는 저장소인데 새 파일을 막는다:\n%s", b)
	}
}
