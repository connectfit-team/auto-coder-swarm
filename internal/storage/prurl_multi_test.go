package storage

import (
	"testing"
	"time"
)

// 값 추가는 저장소를 여럿 고치므로 result 가 "repo: 주소" 여러 줄로 온다.
// 주소가 맨 앞이 아니어서 화면의 "PR 열기" 가 안 떴다.
func TestUpdateTaskStatusFindsURLInMultiRepoResult(t *testing.T) {
	s := testStore(t)
	s.DB.Create(&SwarmTask{ID: "W-multi", Status: StatusRunning, CreatedAt: time.Now()})

	result := "proto-userapis: https://github.com/connectfit-team/proto-userapis/pull/1\n" +
		"gig_mobile_core: https://github.com/connectfit-team/gig_mobile_core/pull/7"
	if err := s.UpdateTaskStatus("W-multi", StatusCompleted, result, ""); err != nil {
		t.Fatalf("갱신 실패: %v", err)
	}
	got, err := s.GetTaskByID("W-multi")
	if err != nil {
		t.Fatalf("조회 실패: %v", err)
	}
	want := "https://github.com/connectfit-team/proto-userapis/pull/1"
	if got.PRURL != want {
		t.Errorf("PRURL = %q, want %q", got.PRURL, want)
	}
	if got.Result != result {
		t.Errorf("result 는 그대로 남아야 한다: %q", got.Result)
	}
}
