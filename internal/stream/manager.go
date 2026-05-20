package stream

import (
	"fmt"
	"net/http"
	"sync"
)

// Thought represents a single step in the agent's reasoning process.
type Thought struct {
	TaskID    uint
	AgentName string
	Message   string
}

// Manager handles real-time broadcasting of agent thoughts to web clients.
type Manager struct {
	subscribers map[uint][]chan Thought
	mu          sync.RWMutex
}

func NewManager() *Manager {
	return &Manager{
		subscribers: make(map[uint][]chan Thought),
	}
}

// Broadcast sends a thought to all clients currently watching a specific task.
func (m *Manager) Broadcast(t Thought) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if subs, ok := m.subscribers[t.TaskID]; ok {
		for _, ch := range subs {
			// Non-blocking send to prevent a slow client from hanging the orchestrator
			select {
			case ch <- t:
			default:
			}
		}
	}
}

// ServeHTTP implements the SSE endpoint for task thought streaming.
func (m *Manager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	taskIDStr := r.URL.Query().Get("id")
	var taskID uint
	fmt.Sscanf(taskIDStr, "%d", &taskID)

	if taskID == 0 {
		http.Error(w, "Invalid Task ID", http.StatusBadRequest)
		return
	}

	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Create a new subscriber channel
	ch := make(chan Thought, 100)
	m.mu.Lock()
	m.subscribers[taskID] = append(m.subscribers[taskID], ch)
	m.mu.Unlock()

	// Ensure cleanup when the client disconnects
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

	// Signal the browser that the connection is established
	fmt.Fprintf(w, "data: {\"message\": \"Connected to Task #%d stream\"}\n\n", taskID)
	flusher.Flush()

	for {
		select {
		case t := <-ch:
			// Use simple JSON-like format for SSE data
			fmt.Fprintf(w, "data: {\"agent\": \"%s\", \"message\": \"%s\"}\n\n", t.AgentName, t.Message)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
