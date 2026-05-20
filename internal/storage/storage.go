package storage

import (
	"fmt"
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
	ID            uint           `gorm:"primaryKey"`
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
}

type RepoLock struct {
	RepoName  string    `gorm:"primaryKey"`
	LockedAt  time.Time
	TaskID    uint
}

type TaskLog struct {
	ID        uint      `gorm:"primaryKey"`
	TaskID    uint      `gorm:"index"`
	Stage     string
	Message   string    `gorm:"type:text"`
	CreatedAt time.Time
}

type Storage struct {
	db *gorm.DB
}

func NewStorage(dbPath string) (*Storage, error) {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil { return nil, err }
	db.AutoMigrate(&SwarmTask{}, &RepoLock{}, &TaskLog{})
	return &Storage{db: db}, nil
}

func (s *Storage) CreateTask(request string) (*SwarmTask, error) {
	task := &SwarmTask{UserRequest: request, Status: StatusPending}
	if err := s.db.Create(task).Error; err != nil { return nil, err }
	return task, nil
}

func (s *Storage) GetTaskByID(id uint) (*SwarmTask, error) {
	var task SwarmTask
	if err := s.db.First(&task, id).Error; err != nil { return nil, err }
	return &task, nil
}

func (s *Storage) GetAllTasks() ([]SwarmTask, error) {
	var tasks []SwarmTask
	err := s.db.Order("created_at desc").Limit(50).Find(&tasks).Error
	return tasks, err
}

func (s *Storage) UpdateTaskRepo(id uint, repoName string) error {
	return s.db.Model(&SwarmTask{}).Where("id = ?", id).Update("repo_name", repoName).Error
}

func (s *Storage) UpdateTaskStatus(id uint, status TaskStatus, result, errLog string) error {
	updates := map[string]interface{}{"status": status, "updated_at": time.Now()}
	if result != "" { updates["result"] = result }
	if errLog != "" { updates["error_log"] = errLog }
	return s.db.Model(&SwarmTask{}).Where("id = ?", id).Updates(updates).Error
}

func (s *Storage) UpdateTaskProposedDiff(id uint, diff string) error {
	return s.db.Model(&SwarmTask{}).Where("id = ?", id).Update("proposed_diff", diff).Error
}

func (s *Storage) UpdateHumanFeedback(id uint, feedback string) error {
	return s.db.Model(&SwarmTask{}).Where("id = ?", id).Update("human_feedback", feedback).Error
}

func (s *Storage) ClaimNextTask() (*SwarmTask, error) {
	var task SwarmTask
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var lockedRepos []string
		tx.Model(&RepoLock{}).Pluck("repo_name", &lockedRepos)

		query := tx.Where("status IN (?)", []TaskStatus{StatusApproved, StatusPending})
		if len(lockedRepos) > 0 {
			query = query.Where("repo_name NOT IN (?)", lockedRepos)
		}

		if err := query.Order("created_at asc").First(&task).Error; err != nil {
			return err
		}

		return tx.Model(&task).Updates(map[string]interface{}{
			"status":     StatusRunning,
			"updated_at": time.Now(),
		}).Error
	})
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (s *Storage) GetNextPendingTask() (*SwarmTask, error) {
	var task SwarmTask
	err := s.db.Where("status IN (?)", []TaskStatus{StatusApproved, StatusPending}).Order("created_at asc").First(&task).Error
	return &task, err
}

func (s *Storage) TryLockRepo(repoName string, taskID uint) (bool, error) {
	if repoName == "" { return true, nil }
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var lock RepoLock
		if err := tx.Where("repo_name = ?", repoName).First(&lock).Error; err == nil {
			if lock.TaskID == taskID { return nil }
			return fmt.Errorf("locked")
		}
		return tx.Create(&RepoLock{RepoName: repoName, LockedAt: time.Now(), TaskID: taskID}).Error
	})
	return err == nil, nil
}

func (s *Storage) UnlockRepo(repoName string) error {
	if repoName == "" { return nil }
	return s.db.Where("repo_name = ?", repoName).Delete(&RepoLock{}).Error
}

func (s *Storage) ResetRunningToPending() error {
	s.db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&RepoLock{})
	return s.db.Model(&SwarmTask{}).Where("status = ?", StatusRunning).Update("status", StatusPending).Error
}

func (s *Storage) AddLog(taskID uint, stage, message string) error {
	log := &TaskLog{
		TaskID:    taskID,
		Stage:     stage,
		Message:   message,
		CreatedAt: time.Now(),
	}
	return s.db.Create(log).Error
}

func (s *Storage) GetLogs(taskID uint) ([]TaskLog, error) {
	var logs []TaskLog
	err := s.db.Where("task_id = ?", taskID).Order("created_at asc").Find(&logs).Error
	return logs, err
}

func (s *Storage) MigrateLogs() {
	s.db.AutoMigrate(&TaskLog{})
}
