package storage

import (
	"fmt"
	"log"
	"math/rand"
	"regexp"
	"time"
)

func (s *Storage) generateWorkID() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return fmt.Sprintf("W-%05d", r.Intn(90000)+10000)
}

func (s *Storage) CreateTask(request string) (*SwarmTask, error) {
	var task *SwarmTask
	for i := 0; i < 5; i++ {
		task = &SwarmTask{
			ID:          s.generateWorkID(),
			UserRequest: request,
			Status:      StatusPending,
		}
		if err := s.DB.Create(task).Error; err == nil {
			log.Printf("[Storage] Task created: %s", task.ID)
			return task, nil
		}
	}
	return nil, fmt.Errorf("failed to generate unique WorkID after 5 attempts")
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

func (s *Storage) GetTodayTasks() ([]SwarmTask, error) {
	var tasks []SwarmTask
	today := time.Now().Truncate(24 * time.Hour)
	err := s.DB.Where("updated_at >= ?", today).Order("updated_at desc").Find(&tasks).Error
	return tasks, err
}

func (s *Storage) UpdateTaskRepo(id string, repoName string) error {
	return s.DB.Model(&SwarmTask{}).Where("id = ?", id).Update("repo_name", repoName).Error
}

func (s *Storage) UpdateTaskStatus(id string, status TaskStatus, result, errLog string) error {
	updates := map[string]interface{}{"status": status, "updated_at": time.Now()}
	if result != "" {
		updates["result"] = result
		// 화면의 "PR 열기" 는 pr_url 만 본다. 여기 안 넣으면 작업은 성공인데
		// 주소는 로그 안에만 남는다.
		//
		// 저장소가 여럿이면 result 가 "repo: 주소" 여러 줄이라 주소가 맨 앞이 아니다.
		if u := urlRe.FindString(result); u != "" {
			updates["pr_url"] = u
		}
	}
	if errLog != "" {
		updates["error_log"] = errLog
	}
	log.Printf("[Storage] Task %s status updated to %s", id, status)
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

func (s *Storage) UpdateCIEWorkID(id string, cieWorkID string) error {
	return s.DB.Model(&SwarmTask{}).Where("id = ?", id).Update("cie_work_id", cieWorkID).Error
}

func (s *Storage) GetContextState(id string) string {
	var task SwarmTask
	if err := s.DB.Select("context_state").First(&task, "id = ?", id).Error; err != nil {
		return ""
	}
	return task.ContextState
}

var urlRe = regexp.MustCompile(`https?://[^\s]+`)
