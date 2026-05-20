package storage

import (
	"fmt"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type TaskStatus string

const (
	StatusPending   TaskStatus = "PENDING"
	StatusRunning   TaskStatus = "RUNNING"
	StatusCompleted TaskStatus = "COMPLETED"
	StatusFailed    TaskStatus = "FAILED"
)

type SwarmTask struct {
	ID          uint           `gorm:"primaryKey"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`
	UserRequest string         `gorm:"type:text"`
	Status      TaskStatus     `gorm:"index"`
	Result      string         `gorm:"type:text"`
	ErrorLog    string         `gorm:"type:text"`
}

type Storage struct {
	db *gorm.DB
}

func NewStorage(dbPath string) (*Storage, error) {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}

	if err := db.AutoMigrate(&SwarmTask{}); err != nil {
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

func (s *Storage) UpdateTaskStatus(id uint, status TaskStatus, result, errLog string) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}
	if result != "" {
		updates["result"] = result
	}
	if errLog != "" {
		updates["error_log"] = errLog
	}
	return s.db.Model(&SwarmTask{}).Where("id = ?", id).Updates(updates).Error
}

func (s *Storage) GetNextPendingTask() (*SwarmTask, error) {
	var task SwarmTask
	err := s.db.Where("status = ?", StatusPending).Order("created_at asc").First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (s *Storage) ResetRunningToPending() error {
	return s.db.Model(&SwarmTask{}).Where("status = ?", StatusRunning).Update("status", StatusPending).Error
}
