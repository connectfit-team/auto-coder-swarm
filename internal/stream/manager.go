package stream

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/connectfit-team/auto-coder-swarm/internal/storage"
)

// Thought represents a single step in the agent's reasoning process.
type Thought struct {
	TaskID    string
	AgentName string
	Message   string
}

// Manager handles real-time broadcasting of agent thoughts to web clients.
type Manager struct {
	subscribers map[string][]chan Thought
	store       *storage.Storage
	mu          sync.RWMutex
}

func NewManager(s *storage.Storage) *Manager {
	return &Manager{
		subscribers: make(map[string][]chan Thought),
		store:       s,
	}
}

// Broadcast sends a thought to all clients currently watching a specific task.
func (m *Manager) Broadcast(t Thought) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if subs, ok := m.subscribers[t.TaskID]; ok {
		for _, ch := range subs {
			select {
			case ch <- t:
			default:
			}
		}
	}
}

// ServeHTTP implements the SSE endpoint for task thought streaming.
func (m *Manager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	taskID := r.URL.Query().Get("id")

	if taskID == "" {
		http.Error(w, "Invalid Task ID", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Fetch historical thoughts from storage
	if m.store != nil {
		history, _ := m.store.GetThoughts(taskID)
		for _, h := range history {
			data := map[string]string{
				"agent":   h.AgentName,
				"message": h.Message,
			}
			b, _ := json.Marshal(data)
			fmt.Fprintf(w, "data: %s\n\n", string(b))
		}
		flusher.Flush()
	}

	ch := make(chan Thought, 500)
	m.mu.Lock()
	m.subscribers[taskID] = append(m.subscribers[taskID], ch)
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		subs := m.subscribers[taskID]
		for i, sub := range subs {
			if sub == ch {
				m.subscribers[taskID] = append(subs[:i], subs[i+1:]...)
				break
			}
		}
		if len(m.subscribers[taskID]) == 0 {
			delete(m.subscribers, taskID)
		}
		m.mu.Unlock()
		close(ch)
	}()

	fmt.Fprintf(w, "data: {\"message\": \"Connected to Task #%s stream (History Loaded)\"}\n\n", taskID)
	flusher.Flush()

	for {
		select {
		case t := <-ch:
			data := map[string]string{
				"agent":   t.AgentName,
				"message": t.Message,
			}
			b, _ := json.Marshal(data)
			fmt.Fprintf(w, "data: %s\n\n", string(b))
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
