package orchestrator

import (
	"strings"
	"testing"
)

func TestParseSteerReadsWhatItCan(t *testing.T) {
	repos := []string{"cms", "worker", "gig_mobile", "proto-userapis"}

	act := ParseSteer("cms 는 빼고 해줘", repos)
	if len(act.Exclude) != 1 || act.Exclude[0] != "cms" {
		t.Errorf("빼라는 말을 못 읽었다: %+v", act)
	}
	if len(act.Only) != 0 {
		t.Errorf("빼라고 했는데 그것만 하라고 읽었다: %+v", act)
	}

	act = ParseSteer("worker 만 고쳐", repos)
	if len(act.Only) != 1 || act.Only[0] != "worker" {
		t.Errorf("그것만 하라는 말을 못 읽었다: %+v", act)
	}

	act = ParseSteer("일단 멈춰줘", repos)
	if !act.Stop {
		t.Error("멈추라는 말을 못 읽었다")
	}

	// 코드로 처리하지 못한 말은 그대로 되돌린다 — 삼키지 않는다.
	act = ParseSteer("라벨은 계약직 말고 기간제로 해줘", repos)
	if !strings.Contains(act.Note, "기간제") {
		t.Errorf("사람의 말을 삼켰다: %+v", act)
	}
}

// 반만 읽고 계획을 바꾸는 것이 가만히 있는 것보다 나쁘다.
func TestParseSteerDoesNotGuess(t *testing.T) {
	repos := []string{"cms", "worker", "gig_mobile"}

	// 이름만 말한 것은 "그것만" 이 아니다.
	act := ParseSteer("worker 에서 빌드 깨질 것 같은데 확인해줘", repos)
	if len(act.Only) > 0 || len(act.Exclude) > 0 {
		t.Errorf("이름만 말한 것을 가리는 말로 읽었다: %+v", act)
	}
	if act.Note == "" {
		t.Error("그 말을 되돌리지 않았다")
	}

	// 뒤집는 말이 붙으면 멈추는 것이 아니다.
	for _, m := range []string{
		"중지하지 말고 계속해",
		"멈추지 마",
		"취소하지 않아도 돼",
	} {
		if ParseSteer(m, repos).Stop {
			t.Errorf("%q 를 멈추라고 읽었다", m)
		}
	}

	// 그래도 멈추라는 말은 멈춘다.
	for _, m := range []string{"일단 멈춰줘", "그만", "stop"} {
		if !ParseSteer(m, repos).Stop {
			t.Errorf("%q 를 못 읽었다", m)
		}
	}
}

func TestKeepSteeredFilters(t *testing.T) {
	repos := []string{"cms", "worker", "gig_mobile"}

	got := KeepSteered(repos, SteerAction{Exclude: []string{"cms"}})
	if strings.Join(got, ",") != "worker,gig_mobile" {
		t.Errorf("뺀 것이 남았다: %v", got)
	}

	got = KeepSteered(repos, SteerAction{Only: []string{"worker"}})
	if strings.Join(got, ",") != "worker" {
		t.Errorf("그것만 남기지 못했다: %v", got)
	}

	// 아무것도 말하지 않았으면 그대로 둔다.
	if got := KeepSteered(repos, SteerAction{}); len(got) != 3 {
		t.Errorf("말하지 않았는데 걸렀다: %v", got)
	}
}
