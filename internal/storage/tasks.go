package storage

import (
	"fmt"
	"math/rand"
	"time"

	"gorm.io/gorm"
)

func (s *Storage) generateWorkID() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return fmt.Sprintf("W-%05d", r.Intn(90000)+10000)
}

func (s *Storage) CreateTask(request string) (*SwarmTask, error) {
	task := &SwarmTask{
		ID:          s.generateWorkID(),
		UserRequest: request,
		Status:      StatusPending,
	}
	if err := s.DB.Create(task).Error; err != nil {
		return nil, err
	}
	return task, nil
}

func (s *Storage) GetTaskByID(id string) (*SwarmTask, error) {
	var task SwarmTask
	if err := s.DB.First(&task, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

func (s *Storage) GetAllTasks() ([]SwarmTask, error) {
	var tasks []SwarmTask
	err := s.DB.Order("created_at desc").Limit(50).Find(&tasks).Error
	return tasks, err
}

func (s *Storage) UpdateTaskRepo(id string, repoName string) error {
	return s.DB.Model(&SwarmTask{}).Where("id = ?", id).Update("repo_name", repoName).Error
}

func (s *Storage) UpdateTaskStatus(id string, status TaskStatus, result, errLog string) error {
	updates := map[string]interface{}{"status": status, "updated_at": time.Now()}
	if result != "" {
		updates["result"] = result
	}
	if errLog != "" {
		updates["error_log"] = errLog
	}
	return s.DB.Model(&SwarmTask{}).Where("id = ?", id).Updates(updates).Error
}

func (s *Storage) UpdateTaskProposedDiff(id string, diff string) error {
	return s.DB.Model(&SwarmTask{}).Where("id = ?", id).Update("proposed_diff", diff).Error
}

func (s *Storage) UpdateHumanFeedback(id string, feedback string) error {
	return s.DB.Model(&SwarmTask{}).Where("id = ?", id).Update("human_feedback", feedback).Error
}

func (s *Storage) UpdateContextState(id string, state string) error {
	return s.DB.Model(&SwarmTask{}).Where("id = ?", id).Update("context_state", state).Error
}

func (s *Storage) GetContextState(id string) string {
	var task SwarmTask
	if err := s.DB.Select("context_state").First(&task, "id = ?", id).Error; err != nil {
		return ""
	}
	return task.ContextState
}

func (s *Storage) ClaimNextTask() (*SwarmTask, error) {
	var task SwarmTask
	err := s.DB.Transaction(func(tx *gorm.DB) error {
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

func (s *Storage) TryLockRepo(repoName string, taskID string) (bool, error) {
	if repoName == "" {
		return true, nil
	}
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		var lock RepoLock
		if err := tx.Where("repo_name = ?", repoName).First(&lock).Error; err == nil {
			if lock.TaskID == taskID {
				return nil
			}
			return fmt.Errorf("locked")
		}
		return tx.Create(&RepoLock{RepoName: repoName, LockedAt: time.Now(), TaskID: taskID}).Error
	})
	return err == nil, nil
}

func (s *Storage) UnlockRepo(repoName string) error {
	if repoName == "" {
		return nil
	}
	return s.DB.Where("repo_name = ?", repoName).Delete(&RepoLock{}).Error
}

func (s *Storage) ResetRunningToPending() error {
	s.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&RepoLock{})
	return s.DB.Model(&SwarmTask{}).Where("status = ?", StatusRunning).Update("status", StatusPending).Error
}
