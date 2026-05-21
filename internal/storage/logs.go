package storage

import (
	"time"
)

func (s *Storage) AddLog(taskID string, stage, message string) error {
	log := &TaskLog{
		TaskID:    taskID,
		Stage:     stage,
		Message:   message,
		CreatedAt: time.Now(),
	}
	return s.DB.Create(log).Error
}

func (s *Storage) AddDeepLog(taskID string, stage, message, prompt, summary string) error {
	log := &TaskLog{
		TaskID:    taskID,
		Stage:     stage,
		Message:   message,
		Prompt:    prompt,
		Summary:   summary,
		CreatedAt: time.Now(),
	}
	return s.DB.Create(log).Error
}

func (s *Storage) GetLogs(taskID string) ([]TaskLog, error) {
	var logs []TaskLog
	err := s.DB.Where("task_id = ?", taskID).Order("created_at asc").Find(&logs).Error
	return logs, err
}

func (s *Storage) AddThought(taskID string, agentName, message string) error {
	thought := &ThoughtLog{
		TaskID:    taskID,
		AgentName: agentName,
		Message:   message,
		CreatedAt: time.Now(),
	}
	
	// Non-blocking send to async queue
	select {
	case s.thoughtQueue <- thought:
	default:
		// Queue full, fallback to sync or skip (logging failure)
	}
	return nil
}

func (s *Storage) GetThoughts(taskID string) ([]ThoughtLog, error) {
	var thoughts []ThoughtLog
	err := s.DB.Where("task_id = ?", taskID).Order("created_at asc").Find(&thoughts).Error
	return thoughts, err
}

func (s *Storage) MigrateLogs() {
	s.DB.AutoMigrate(&TaskLog{}, &ThoughtLog{}, &Setting{}, &SwarmTask{})
}
