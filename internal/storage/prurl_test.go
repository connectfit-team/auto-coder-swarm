package storage

import (
	"testing"
	"time"
)

func TestUpdateTaskStatusSavesPRURL(t *testing.T) {
	s := testStore(t)
	s.DB.Create(&SwarmTask{ID: "W-pr", Status: StatusRunning, CreatedAt: time.Now()})

	url := "https://github.com/connectfit-team/cms/pull/192"
	if err := s.UpdateTaskStatus("W-pr", StatusCompleted, url, ""); err != nil {
		t.Fatalf("갱신 실패: %v", err)
	}
	got, err := s.GetTaskByID("W-pr")
	if err != nil {
		t.Fatalf("조회 실패: %v", err)
	}
	// 화면의 "PR 열기" 가 이 필드를 본다.
	if got.PRURL != url {
		t.Errorf("PRURL = %q, want %q", got.PRURL, url)
	}
}

func TestUpdateTaskStatusIgnoresNonURLResult(t *testing.T) {
	s := testStore(t)
	s.DB.Create(&SwarmTask{ID: "W-msg", Status: StatusRunning, CreatedAt: time.Now()})
	if err := s.UpdateTaskStatus("W-msg", StatusCompleted, "그냥 결과 문구", ""); err != nil {
		t.Fatalf("갱신 실패: %v", err)
	}
	got, _ := s.GetTaskByID("W-msg")
	if got.PRURL != "" {
		t.Errorf("URL 이 아닌데 PRURL 에 넣었다: %q", got.PRURL)
	}
}
