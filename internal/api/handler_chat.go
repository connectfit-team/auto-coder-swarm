package api

import (
	"encoding/json"
	"net/http"

	"github.com/connectfit-team/auto-coder-swarm/internal/orchestrator"
)

// HandleChatSubmission provides a specialized API endpoint for chat interfaces.
// It wraps the task submission logic but could be expanded for session-based chat.
func (h *SwarmHandler) HandleChatSubmission(w http.ResponseWriter, r *http.Request) {
	if !h.checkAuth(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Translate chat message to a Swarm Task
	statelessReq := orchestrator.StatelessRequest{
		UserRequest: req.Message,
	}

	b, _ := json.Marshal(statelessReq)
	task, err := h.store.CreateTask(string(b))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"task_id": task.ID,
		"status":  "PENDING",
		"message": "Task created from chat",
	})
}
