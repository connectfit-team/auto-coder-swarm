package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/connectfit-team/auto-coder-swarm/internal/orchestrator"
	"github.com/connectfit-team/auto-coder-swarm/internal/storage"
)

func (h *SwarmHandler) HandleListTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := h.store.GetAllTasks()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

func (h *SwarmHandler) HandleGetTask(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	task, err := h.store.GetTaskByID(id)
	if err != nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}
	
	logs, _ := h.store.GetLogs(id)
	thoughts, _ := h.store.GetThoughts(id)
	
	response := map[string]interface{}{
		"task":     task,
		"logs":     logs,
		"thoughts": thoughts,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *SwarmHandler) HandleSubmitTask(w http.ResponseWriter, r *http.Request) {
	if !h.checkAuth(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	var req orchestrator.StatelessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	b, _ := json.Marshal(req)
	task, err := h.store.CreateTask(string(b))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"task_id": task.ID, "status": "PENDING"})
}

func (h *SwarmHandler) HandleStopTask(w http.ResponseWriter, r *http.Request) {
	if !h.checkAuth(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	id := r.URL.Query().Get("id")
	
	// [Deep Stop] Stop remote CIE task if tracked
	task, err := h.store.GetTaskByID(id)
	if err == nil && task.CIEWorkID != "" {
		log.Printf("[API] Stopping remote CIE task: %s", task.CIEWorkID)
		h.insight.StopTask(r.Context(), task.CIEWorkID)
	}

	if h.worker.Stop(id) {
		h.store.UpdateTaskStatus(id, storage.StatusCancelled, "", "Stopped via API")
		fmt.Fprintf(w, "Task %s stopped", id)
	} else {
		http.Error(w, "Task not running or not found", http.StatusNotFound)
	}
}
