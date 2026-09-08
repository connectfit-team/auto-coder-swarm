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
