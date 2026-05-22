package storage

import (
	"fmt"
	"time"
	"log"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *Storage) ClaimNextTask() (*SwarmTask, error) {
	var task SwarmTask
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		// Optimization: Single query with NOT IN (Subquery) and Row Locking for MariaDB/MySQL
		query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("status IN (?)", []TaskStatus{StatusApproved, StatusPending}).
			Where("repo_name IS NULL OR repo_name NOT IN (SELECT repo_name FROM repo_locks)")

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
	log.Printf("[Storage] Task %s claimed by worker", task.ID)
	return &task, nil
}

func (s *Storage) TryLockRepo(repoName string, taskID string) (bool, error) {
	if repoName == "" {
		return true, nil
	}
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		var lock RepoLock
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("repo_name = ?", repoName).First(&lock).Error; err == nil {
			if lock.TaskID == taskID {
				return nil
			}
			return fmt.Errorf("locked by task %s", lock.TaskID)
		}
		log.Printf("[Storage] Locking repo %s for task %s", repoName, taskID)
		return tx.Create(&RepoLock{RepoName: repoName, LockedAt: time.Now(), TaskID: taskID}).Error
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *Storage) UnlockRepo(repoName string) error {
	if repoName == "" {
		return nil
	}
	log.Printf("[Storage] Unlocking repo %s", repoName)
	return s.DB.Where("repo_name = ?", repoName).Delete(&RepoLock{}).Error
}

func (s *Storage) ResetRunningToPending() error {
	log.Println("[Storage] Resetting all running tasks to pending and clearing locks")
	s.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&RepoLock{})
	return s.DB.Model(&SwarmTask{}).Where("status = ?", StatusRunning).Update("status", StatusPending).Error
}
