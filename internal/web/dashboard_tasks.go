package web

import (
	"net/http"
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/storage"
)

func (h *DashboardHandler) HandleHome(w http.ResponseWriter, r *http.Request) {
	tasks, _ := h.store.GetAllTasks()
	h.render(w, "home.html", map[string]interface{}{"Tasks": tasks})
}

func (h *DashboardHandler) HandleTaskDetail(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	task, _ := h.store.GetTaskByID(id)
	rawLogs, _ := h.store.GetLogs(id)

	var uiLogs []UISafeLog
	for _, l := range rawLogs {
		uiLogs = append(uiLogs, UISafeLog{
			CreatedAt:  l.CreatedAt.Format("2006-01-02 15:04:05"),
			Stage:      l.Stage,
			StageLower: strings.ToLower(l.Stage),
			Message:    l.Message,
			Prompt:     l.Prompt,
			Summary:    l.Summary,
		})
	}

	h.render(w, "task_detail.html", map[string]interface{}{
		"Task": task,
		"Logs": uiLogs,
	})
}

func (h *DashboardHandler) HandleStopTask(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	h.store.UpdateTaskStatus(id, storage.StatusCancelled, "", "Stopping...")
	h.worker.Stop(id)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *DashboardHandler) HandleApproveTask(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	h.store.UpdateTaskStatus(id, storage.StatusApproved, "", "")
	http.Redirect(w, r, "/task?id="+id, http.StatusSeeOther)
}

func (h *DashboardHandler) HandleRejectTask(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	feedback := r.FormValue("feedback")
	h.store.UpdateHumanFeedback(id, feedback)
	h.store.UpdateTaskStatus(id, storage.StatusPending, "", "Rejected by user")
	http.Redirect(w, r, "/task?id="+id, http.StatusSeeOther)
}
