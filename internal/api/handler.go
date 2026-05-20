package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/connectfit-team/auto-coder-swarm/internal/orchestrator"
	"github.com/connectfit-team/auto-coder-swarm/internal/storage"
)

type SwarmHandler struct {
	store *storage.Storage
}

func NewSwarmHandler(s *storage.Storage) *SwarmHandler {
	return &SwarmHandler{store: s}
}

type TaskResponse struct {
	TaskID uint   `json:"task_id"`
	Status string `json:"status"`
}

type StatusResponse struct {
	TaskID    uint   `json:"task_id"`
	Status    string `json:"status"`
	Result    string `json:"result,omitempty"`
	ErrorLog  string `json:"error_log,omitempty"`
	UpdatedAt string `json:"updated_at"`
}

func (h *SwarmHandler) checkAuth(r *http.Request) bool {
	expectedKey := os.Getenv("SWARM_API_KEY")
	if expectedKey == "" {
		return true // Dev mode: No key required if env not set
	}
	clientKey := r.Header.Get("X-API-Key")
	return clientKey == expectedKey
}

func (h *SwarmHandler) HandleSubmitTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.checkAuth(r) {
		http.Error(w, "Unauthorized: Invalid or missing X-API-Key", http.StatusUnauthorized)
		return
	}
	var req orchestrator.StatelessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}
	b, _ := json.Marshal(req)
	task, err := h.store.CreateTask(string(b))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(TaskResponse{TaskID: task.ID, Status: string(storage.StatusPending)})
}

func (h *SwarmHandler) HandleGetStatus(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	task, err := h.store.GetTaskByID(uint(id))
	if err != nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(StatusResponse{
		TaskID:    task.ID,
		Status:    string(task.Status),
		Result:    task.Result,
		ErrorLog:  task.ErrorLog,
		UpdatedAt: task.UpdatedAt.Format("2006-01-02 15:04:05"),
	})
}

func (h *SwarmHandler) HandleApproveTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.checkAuth(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	idStr := r.URL.Query().Get("id")
	id, _ := strconv.ParseUint(idStr, 10, 32)

	err := h.store.UpdateTaskStatus(uint(id), storage.StatusApproved, "", "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Task %d approved and queued for PR generation", id)
}

func (h *SwarmHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/tasks", h.HandleSubmitTask)
	mux.HandleFunc("GET /api/v1/status", h.HandleGetStatus)
	mux.HandleFunc("POST /api/v1/approve", h.HandleApproveTask)
}
