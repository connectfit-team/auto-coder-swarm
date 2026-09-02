package storage

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func testStore(t *testing.T) *Storage {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Skipf("sqlite 없음: %v", err)
	}
	if err := db.AutoMigrate(&SwarmTask{}, &RepoLock{}); err != nil {
		t.Skipf("마이그레이션 실패: %v", err)
	}
	db.Where("1 = 1").Delete(&SwarmTask{})
	return &Storage{DB: db}
}

func TestClaimNextTaskKeepsApprovedStatus(t *testing.T) {
	s := testStore(t)
	// 사람이 승인한 작업.
	s.DB.Create(&SwarmTask{ID: "W-1", Status: StatusApproved, CreatedAt: time.Now()})

	got, err := s.ClaimNextTask()
	if err != nil {
		t.Fatalf("집지 못했다: %v", err)
	}
	// 집는 순간 status 는 RUNNING 으로 덮인다.
	if got.Status != StatusRunning {
		t.Errorf("status = %v, want RUNNING", got.Status)
	}
	// **집기 전 상태가 남아야 한다.** 이게 없어서 승인이 한 번도 통하지 않았다.
	if got.ClaimedStatus != StatusApproved {
		t.Errorf("ClaimedStatus = %v, want APPROVED — 승인이 사라지면 PR 을 못 만든다", got.ClaimedStatus)
	}
}

func TestClaimNextTaskPendingIsNotApproved(t *testing.T) {
	s := testStore(t)
	s.DB.Create(&SwarmTask{ID: "W-2", Status: StatusPending, CreatedAt: time.Now()})
	got, err := s.ClaimNextTask()
	if err != nil {
		t.Fatalf("집지 못했다: %v", err)
	}
	if got.ClaimedStatus == StatusApproved {
		t.Error("승인하지 않은 작업이 승인된 것으로 보인다")
	}
}
