package gitmgr

import (
	"os"
	"testing"
)

// 계약 저장소에는 무슨 설정이든 밀지 않는다.
//
// 계약은 앱·웹·서버가 함께 쓴다. 잘못 밀리면 한 곳이 아니라 전부 깨진다.
// 허용 목록에 실수로 들어가는 것만으로 사고가 나서는 안 된다.
func TestContractReposAreNeverPushable(t *testing.T) {
	for _, allow := range []string{"", "*", "protogen", "proto-ceowebapis", "gig_ceo_web,protogen"} {
		t.Setenv("SWARM_PUSH_ALLOW", allow)
		for _, repo := range []string{"protogen", "proto-ceowebapis", "proto-userapis", "PROTOGEN", "Proto-CommonAPIs"} {
			if ok, why := pushAllowed(repo); ok {
				t.Errorf("SWARM_PUSH_ALLOW=%q 에서 %s 로 밀 수 있다고 했다 (%s)", allow, repo, why)
			}
		}
	}
}

// 그 밖의 저장소는 여전히 목록을 따른다.
func TestOtherReposStillFollowTheAllowList(t *testing.T) {
	t.Setenv("SWARM_PUSH_ALLOW", "")
	if ok, _ := pushAllowed("some-repo"); ok {
		t.Error("목록이 비었는데 밀 수 있다고 했다")
	}
	t.Setenv("SWARM_PUSH_ALLOW", "some-repo")
	if ok, why := pushAllowed("some-repo"); !ok {
		t.Errorf("목록에 있는데 막았다: %s", why)
	}
	_ = os.Unsetenv("SWARM_PUSH_ALLOW")
}
