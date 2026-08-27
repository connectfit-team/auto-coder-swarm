package storage

import (
	"os"
	"testing"
)

func TestStorageTasks(t *testing.T) {
	dbPath := "./test_swarm.db"
	defer os.Remove(dbPath)

	s, err := NewStorage(dbPath, nil)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	task, err := s.CreateTask("test request")
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	if task.ID == "" {
		t.Error("Task ID should not be empty")
	}

	retrieved, err := s.GetTaskByID(task.ID)
	if err != nil {
		t.Fatalf("Failed to get task: %v", err)
	}

	if retrieved.UserRequest != "test request" {
		t.Errorf("Expected request 'test request', got '%s'", retrieved.UserRequest)
	}

	err = s.UpdateTaskStatus(task.ID, StatusCompleted, "result", "")
	if err != nil {
		t.Fatalf("Failed to update status: %v", err)
	}

	updated, _ := s.GetTaskByID(task.ID)
	if updated.Status != StatusCompleted {
		t.Errorf("Expected status COMPLETED, got %s", updated.Status)
	}
}
