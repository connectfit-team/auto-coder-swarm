package storage

import (
	"fmt"
	"log"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type TaskStatus string

const (
	StatusPending          TaskStatus = "PENDING"
	StatusRunning          TaskStatus = "RUNNING"
	StatusWaitingApproval TaskStatus = "WAITING_APPROVAL"
	StatusApproved         TaskStatus = "APPROVED"
	StatusCompleted        TaskStatus = "COMPLETED"
	StatusFailed           TaskStatus = "FAILED"
)

type SwarmTask struct {
	ID          uint           `gorm:"primaryKey"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`
	UserRequest string         `gorm:"type:text"`
	RepoName    string         `gorm:"index"`
	Status      TaskStatus     `gorm:"index"`
	Result      string         `gorm:"type:text"`
	ErrorLog    string         `gorm:"type:text"`
}

type RepoLock struct {
	RepoName  string    `gorm:"primaryKey"`
	LockedAt  time.Time
	TaskID    uint
}

type Storage struct {
	db *gorm.DB
}

func NewStorage(dbPath string) (*Storage, error) {
	log.Printf("[Storage] Initializing database at: %s", dbPath)
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}

	if err := db.AutoMigrate(&SwarmTask{}, &RepoLock{}); err != nil {
		return nil, fmt.Errorf("failed to migrate schema: %w", err)
	}

	return &Storage{db: db}, nil
}

func (s *Storage) CreateTask(request string) (*SwarmTask, error) {
	task := &SwarmTask{
		UserRequest: request,
		Status:      StatusPending,
	}
	if err := s.db.Create(task).Error; err != nil {
		return nil, err
	}
	return task, nil
}

func (s *Storage) GetTaskByID(id uint) (*SwarmTask, error) {
	var task SwarmTask
	if err := s.db.First(&task, id).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

func (s *Storage) UpdateTaskRepo(id uint, repoName string) error {
	return s.db.Model(&SwarmTask{}).Where("id = ?", id).Update("repo_name", repoName).Error
}

func (s *Storage) UpdateTaskStatus(id uint, status TaskStatus, result, errLog string) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}
	if result != "" { updates["result"] = result }
	if errLog != "" { updates["error_log"] = errLog }
	return s.db.Model(&SwarmTask{}).Where("id = ?", id).Updates(updates).Error
}

func (s *Storage) GetNextPendingTask() (*SwarmTask, error) {
	var task SwarmTask
	// Priority: APPROVED tasks should be resumed first, then PENDING
	err := s.db.Where("status IN (?)", []TaskStatus{StatusApproved, StatusPending}).Order("created_at asc").First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (s *Storage) TryLockRepo(repoName string, taskID uint) (bool, error) {
	if repoName == "" { return true, nil }
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var lock RepoLock
		if err := tx.Where("repo_name = ?", repoName).First(&lock).Error; err == nil {
			if lock.TaskID == taskID { return nil } // Already locked by us
			return fmt.Errorf("repo already locked")
		}
		return tx.Create(&RepoLock{RepoName: repoName, LockedAt: time.Now(), TaskID: taskID}).Error
	})
	if err != nil { return false, nil }
	return true, nil
}

func (s *Storage) UnlockRepo(repoName string) error {
	if repoName == "" { return nil }
	return s.db.Where("repo_name = ?", repoName).Delete(&RepoLock{}).Error
}

func (s *Storage) ResetRunningToPending() error {
	s.db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&RepoLock{})
	return s.db.Model(&SwarmTask{}).Where("status = ?", StatusRunning).Update("status", StatusPending).Error
}
