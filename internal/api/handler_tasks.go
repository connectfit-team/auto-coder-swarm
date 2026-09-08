package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

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

	// 한 요청이 저장소 여럿에 걸치면 저장소마다 작업이 하나씩 생긴다.
	// 그것들을 함께 줘야 화면이 PR 단추를 여럿 띄울 수 있다 — 그러지 않아
	// 지금까지 단추가 하나뿐이었다.
	root := id
	if task.ParentTaskID != "" {
		root = task.ParentTaskID
	}
	family, _ := h.store.ChildTasks(root)
	if root != id {
		if p, err := h.store.GetTaskByID(root); err == nil {
			family = append([]storage.SwarmTask{*p}, family...)
		}
	}

	response := map[string]interface{}{
		"task":     task,
		"logs":     logs,
		"thoughts": thoughts,
		// 이 작업과 형제 작업들. 저장소마다 하나씩이고 PR 도 저마다 있다.
		"family": family,
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

// HandleSteerTask 는 도는 도중에 사람이 보낸 지시를 큐에 넣는다.
//
// **받았다는 것이 반영했다는 뜻은 아니다.** 이미 열린 PR 은 열린 채로 남고,
// 시작된 검사는 멈추지 않는다. 다음 걸음부터 반영한다. 그래서 202 를 준다 —
// 200 은 "했다" 로 읽힌다.
//
// 지시는 DB 에 남긴다. 이 기계는 하루 세 번 다시 뜨고 작업은 몇 분씩 도는데,
// 메모리에만 두면 그 사이 사라진다. 사라진 지시는 없는 것과 같은데 사람은
// 말했다고 여긴다.
func (h *SwarmHandler) HandleSteerTask(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	var body struct {
		Message string `json:"message"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	msg := strings.TrimSpace(body.Message)
	if id == "" || msg == "" {
		http.Error(w, "id 와 message 가 있어야 합니다", http.StatusBadRequest)
		return
	}

	t, err := h.store.GetTaskByID(id)
	if err != nil {
		http.Error(w, "그런 작업이 없습니다", http.StatusNotFound)
		return
	}
	if finishedStatus(t.Status) {
		w.WriteHeader(http.StatusConflict)
		fmt.Fprintf(w, "작업 %s 는 이미 %s 입니다 — 끝난 작업에는 방향을 더할 수 없습니다", id, t.Status)
		return
	}

	st, err := h.store.AddSteer(id, msg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"queued":   true,
		"steer_id": st.ID,
		"note":     "다음 걸음부터 반영합니다. 이미 열린 PR 은 그대로 남습니다.",
	})
}

// finishedStatus 는 더 이상 도는 중이 아닌 상태인지 본다.
func finishedStatus(s storage.TaskStatus) bool {
	switch s {
	case storage.StatusCompleted, storage.StatusFailed, storage.StatusCancelled:
		return true
	}
	return false
}
