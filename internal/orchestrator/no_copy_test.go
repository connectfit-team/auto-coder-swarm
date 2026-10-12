package orchestrator

import (
	"os"
	"strings"
	"testing"
)

// 본보기를 **베껴 넣는** 것을 막는다.
//
// 실측 W-36577 이 본보기로 보여 준 `RequestCreateTrade`·`ResponseCreateTrade`·
// `RequestDeleteEndorsement`·`ResponseDeleteEndorsement` 를 **계약에 그대로
// 넣었다.** 넷 다 이미 있는 메시지다. 검토자가 「요청에 없던 것들」 이라고
// 짚었다. 보여 주는 것이 값이 있었던 만큼, 보여 준 것을 베끼는 것도 막아야 한다.
func TestExamplesSayDoNotCopy(t *testing.T) {
	repo := "/home/cnf/cie-repos/proto-ceowebapis"
	if _, err := os.Stat(repo); err != nil {
		t.Skip("계약 사본이 없다")
	}
	blocks := map[string]string{
		"요청·응답 짝": protoConventionExample(repo, "ceoweb/v1/connect.service.proto", 2),
		"상태 담는 꼴": protoStateFieldExample(repo),
	}
	if bad := emptyNewMessage(repo, "+++ b/x.proto\n+message ResponseX {\n+}\n"); len(bad) > 0 {
		blocks["빈 메시지 쪽지"] = strings.Join(bad[0].Evidence, "\n")
	}
	for name, b := range blocks {
		if b == "" {
			t.Errorf("%s: 본보기가 비었다 — 이 시험이 아무것도 재지 못한다", name)
			continue
		}
		if !strings.Contains(b, "베껴 넣지 마라") {
			t.Errorf("%s: 베끼지 말라는 말이 없다:\n%s", name, b[:min(len(b), 300)])
		}
	}
}

// 「그대로 본떠라」 는 「그대로 넣어라」 로 읽힌다 — 그 말을 쓰지 않는다.
func TestExamplesAvoidCopyWording(t *testing.T) {
	repo := "/home/cnf/cie-repos/proto-ceowebapis"
	if _, err := os.Stat(repo); err != nil {
		t.Skip("계약 사본이 없다")
	}
	for _, b := range []string{
		protoConventionExample(repo, "ceoweb/v1/connect.service.proto", 2),
		protoStateFieldExample(repo),
	} {
		for _, bad := range []string{"그대로 본떠라", "그대로 따라라", "이렇게 채운다]"} {
			if strings.Contains(b, bad) {
				t.Errorf("%q 가 아직 있다", bad)
			}
		}
	}
}
