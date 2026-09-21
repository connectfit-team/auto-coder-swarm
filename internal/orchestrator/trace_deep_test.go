package orchestrator

import "testing"

// 계획이 화면 파일만 짚은 회차에서는 gRPC 를 부르는 곳이 없어 고리가 끊겼다
// (W-58547). 화면은 데이터 모듈을 들여오고, 그 모듈이 공장을 부른다.
func TestProtoOwnerViaClientDeep(t *testing.T) {
	const repo = "/home/cnf/cie-repos/gig_ceo_web"
	// 데이터 모듈에서는 한 걸음도 안 가고 찾는다.
	if o, _, _ := protoOwnerViaClientDeep(repo, "src/lib/server/data/connectcud.ts", 2); o != "proto-ceowebapis" {
		t.Skipf("사본이 없거나 모양이 바뀌었다: %q", o)
	}
	// 화면 파일에서도 들여온 모듈을 타고 닿아야 한다.
	o, _, why := protoOwnerViaClientDeep(repo, "src/routes/(auth)/store/connect/+page.server.ts", 2)
	if o != "proto-ceowebapis" {
		t.Errorf("화면 파일에서 못 닿았다: %q (%s)", o, why)
	}
}
