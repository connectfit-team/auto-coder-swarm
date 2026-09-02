package storage

import (
	"time"

	"gorm.io/gorm"
)

type TaskStatus string

const (
	StatusPending         TaskStatus = "PENDING"
	StatusRunning         TaskStatus = "RUNNING"
	StatusWaitingApproval TaskStatus = "WAITING_APPROVAL"
	StatusApproved        TaskStatus = "APPROVED"
	StatusCompleted       TaskStatus = "COMPLETED"
	StatusFailed          TaskStatus = "FAILED"
	StatusCancelled       TaskStatus = "CANCELLED"
)

type SwarmTask struct {
	// 워커가 집을 때의 상태. DB 에 저장하지 않는다 — 집는 순간 status 는
	// RUNNING 으로 덮이므로, 그 전에 APPROVED 였는지를 여기 담아 둔다.
	ClaimedStatus TaskStatus `gorm:"-"`

	ID          string `gorm:"primaryKey"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`
	UserRequest string         `gorm:"type:text"`
	RepoName    string         `gorm:"index"`
	Status      TaskStatus     `gorm:"index"`
	Result      string         `gorm:"type:text"`
	// 만들어진 PR 주소. Result 에도 같은 값이 들어가지만, 화면이 "PR 열기"
	// 단추를 띄우려면 이름 있는 자리가 필요하다 — Result 는 성공 문구가 올
	// 수도 있는 자리라 그것만 보고 링크를 만들 수 없다.
	PRURL         string `gorm:"column:pr_url;type:text"`
	ErrorLog      string `gorm:"type:text"`
	ProposedDiff  string `gorm:"type:text"`
	HumanFeedback string `gorm:"type:text"`
	ContextState  string `gorm:"type:text"`
	CIEWorkID     string `gorm:"index"`
}

type RepoLock struct {
	RepoName string `gorm:"primaryKey"`
	LockedAt time.Time
	TaskID   string
}

type TaskLog struct {
	ID        uint   `gorm:"primaryKey"`
	TaskID    string `gorm:"index"`
	Stage     string
	Message   string `gorm:"type:text"`
	Prompt    string `gorm:"type:text"`
	Summary   string `gorm:"type:text"`
	CreatedAt time.Time
}

type ThoughtLog struct {
	ID        uint   `gorm:"primaryKey"`
	TaskID    string `gorm:"index"`
	AgentName string
	Message   string `gorm:"type:text"`
	CreatedAt time.Time
}

type Setting struct {
	Key   string `gorm:"primaryKey"`
	Value string `gorm:"type:text"`
}
