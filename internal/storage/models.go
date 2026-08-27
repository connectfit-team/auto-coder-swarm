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
	ID            string `gorm:"primaryKey"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     gorm.DeletedAt `gorm:"index"`
	UserRequest   string         `gorm:"type:text"`
	RepoName      string         `gorm:"index"`
	Status        TaskStatus     `gorm:"index"`
	Result        string         `gorm:"type:text"`
	ErrorLog      string         `gorm:"type:text"`
	ProposedDiff  string         `gorm:"type:text"`
	HumanFeedback string         `gorm:"type:text"`
	ContextState  string         `gorm:"type:text"`
	CIEWorkID     string         `gorm:"index"`
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
