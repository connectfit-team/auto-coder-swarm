package gitmgr

import "testing"

func TestPushAllowed(t *testing.T) {
	// 비어 있으면 아무 데도 못 민다. 잊으면 안전해지는 쪽이다.
	t.Setenv("SWARM_PUSH_ALLOW", "")
	if ok, why := pushAllowed("cms"); ok {
		t.Errorf("비어 있는데 밀도록 했다: %s", why)
	}

	t.Setenv("SWARM_PUSH_ALLOW", "cms, worker")
	if ok, _ := pushAllowed("cms"); !ok {
		t.Error("적힌 저장소를 막았다")
	}
	if ok, _ := pushAllowed("worker"); !ok {
		t.Error("공백이 섞인 항목을 못 읽었다")
	}
	if ok, _ := pushAllowed("gig_ceo_web"); ok {
		t.Error("안 적힌 저장소로 밀도록 했다")
	}

	t.Setenv("SWARM_PUSH_ALLOW", "*")
	if ok, _ := pushAllowed("아무거나"); !ok {
		t.Error("* 인데 막았다")
	}
}
