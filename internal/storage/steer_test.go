package storage

import (
	"os"
	"testing"
)

// 도중에 들어온 지시는 DB 에 남아야 한다.
//
// 이 기계는 하루 세 번 다시 뜨고 작업은 몇 분씩 돈다 — 메모리에만 두면 그
// 사이에 사라진다. 사라진 지시는 없는 것과 같은데 사람은 말했다고 여긴다.
func TestSteerQueueSurvivesRestart(t *testing.T) {
	dbPath := "./test_steer.db"
	defer os.Remove(dbPath)

	s, err := NewStorage("", dbPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	task, err := s.CreateTask("소셜로그인에 instagram 추가")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.AddSteer(task.ID, "cms 는 빼고"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddSteer(task.ID, "worker 만"); err != nil {
		t.Fatal(err)
	}

	// 다시 뜬 것처럼 새로 연다. 지시가 그대로 있어야 한다.
	s2, err := NewStorage("", dbPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	pending, err := s2.PendingSteers(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 2 {
		t.Fatalf("다시 뜬 뒤 지시가 %d개다 — 둘이어야 한다", len(pending))
	}
	// 오래된 것부터 온다. 사람이 말한 순서가 뜻을 바꾼다.
	if pending[0].Message != "cms 는 빼고" {
		t.Errorf("순서가 뒤바뀌었다: %q", pending[0].Message)
	}

	if err := s2.MarkSteersApplied([]uint{pending[0].ID}); err != nil {
		t.Fatal(err)
	}
	left, err := s2.PendingSteers(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 1 || left[0].Message != "worker 만" {
		t.Errorf("집어 간 것이 다시 나왔다: %+v", left)
	}

	// 집어 간 것도 세는 데는 들어간다 — 화면이 "몇 개 받았다" 를 적는다.
	n, err := s2.CountSteers(task.ID)
	if err != nil || n != 2 {
		t.Errorf("받은 지시 수: %d (%v)", n, err)
	}
}
