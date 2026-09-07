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

// HandleApproveTask 는 사람의 승인을 받아 작업을 다시 큐에 넣는다.
// 그 뒤 실행은 사람이 본 diff 를 그대로 올린다.
func (h *SwarmHandler) HandleApproveTask(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	// **이미 도는 작업에 승인해도 소용이 없다.**
	//
	// 상태만 APPROVED 로 바꿔 놓으면, 지금 도는 실행은 그 사실을 모른 채
	// 끝나면서 다시 WAITING_APPROVAL 로 덮어쓴다. 승인이 조용히 사라진다.
	// 무엇이 일어났는지 말해 준다.
	if t, err := h.store.GetTaskByID(id); err == nil && t.Status == storage.StatusRunning {
		w.WriteHeader(http.StatusConflict)
		fmt.Fprintf(w, "작업 %s 는 지금 돌고 있습니다. 끝난 뒤에 다시 승인하세요.", id)
		return
	}
	if err := h.store.UpdateTaskStatus(id, storage.StatusApproved, "", ""); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "Task %s approved", id)
}
