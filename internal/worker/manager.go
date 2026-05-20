package worker

import (
	"context"
	"sync"
)

type Manager struct {
	cancels map[uint]context.CancelFunc
	mu      sync.Mutex
}

func NewManager() *Manager {
	return &Manager{
		cancels: make(map[uint]context.CancelFunc),
	}
}

func (m *Manager) Register(taskID uint, cancel context.CancelFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cancels[taskID] = cancel
}

func (m *Manager) Unregister(taskID uint) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.cancels, taskID)
}

func (m *Manager) Stop(taskID uint) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cancel, ok := m.cancels[taskID]; ok {
		cancel()
		delete(m.cancels, taskID)
		return true
	}
	return false
}
