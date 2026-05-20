package web

import (
	"html/template"
	"net/http"
	"path/filepath"
	"log"
	"os"
	"os/exec"
	"strconv"
	"github.com/connectfit-team/auto-coder-swarm/internal/storage"
	"github.com/connectfit-team/auto-coder-swarm/internal/worker"
)

type DashboardHandler struct {
	store    *storage.Storage
	worker   *worker.Manager
	tmplPath string
}

func NewDashboardHandler(s *storage.Storage, w *worker.Manager, tmplPath string) *DashboardHandler {
	return &DashboardHandler{store: s, worker: w, tmplPath: tmplPath}
}

func (h *DashboardHandler) render(w http.ResponseWriter, page string, data interface{}) {
	layout := filepath.Join(h.tmplPath, "layout.html")
	content := filepath.Join(h.tmplPath, page)

	tmpl, err := template.ParseFiles(layout, content)
	if err != nil {
		log.Printf("[Web] Template error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = tmpl.ExecuteTemplate(w, "layout.html", data)
	if err != nil {
		log.Printf("[Web] Execution error: %v", err)
	}
}

func (h *DashboardHandler) HandleHome(w http.ResponseWriter, r *http.Request) {
	tasks, _ := h.store.GetAllTasks()
	data := map[string]interface{}{
		"Tasks": tasks,
	}
	h.render(w, "home.html", data)
}

func (h *DashboardHandler) HandleTaskDetail(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	idInt, _ := strconv.Atoi(idStr)
	task, _ := h.store.GetTaskByID(uint(idInt))
	logs, _ := h.store.GetLogs(uint(idInt))
	data := map[string]interface{}{
		"Task": task,
		"Logs": logs,
	}
	h.render(w, "task_detail.html", data)
}

func (h *DashboardHandler) HandleStopTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	idStr := r.URL.Query().Get("id")
	idInt, _ := strconv.Atoi(idStr)
	stopped := h.worker.Stop(uint(idInt))
	if stopped {
		h.store.UpdateTaskStatus(uint(idInt), storage.StatusFailed, "", "Stopped by user")
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *DashboardHandler) HandleApproveTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	idStr := r.URL.Query().Get("id")
	idInt, _ := strconv.Atoi(idStr)
	h.store.UpdateTaskStatus(uint(idInt), storage.StatusApproved, "", "")
	http.Redirect(w, r, "/task?id="+idStr, http.StatusSeeOther)
}

func (h *DashboardHandler) HandleRejectTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	idStr := r.URL.Query().Get("id")
	idInt, _ := strconv.Atoi(idStr)
	feedback := r.FormValue("feedback")
	h.store.UpdateHumanFeedback(uint(idInt), feedback)
	h.store.UpdateTaskStatus(uint(idInt), storage.StatusPending, "", "Rejected by user")
	http.Redirect(w, r, "/task?id="+idStr, http.StatusSeeOther)
}

func (h *DashboardHandler) HandleProjects(w http.ResponseWriter, r *http.Request) {
	data, _ := os.ReadFile("/home/cnf/projects/auto-coder-swarm/PROJECTS.md")
	h.render(w, "projects.html", map[string]interface{}{"Specs": string(data)})
}

func (h *DashboardHandler) HandleUpdateProjects(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	content := r.FormValue("content")
	os.WriteFile("/home/cnf/projects/auto-coder-swarm/PROJECTS.md", []byte(content), 0644)
	http.Redirect(w, r, "/projects", http.StatusSeeOther)
}

func (h *DashboardHandler) HandleProgress(w http.ResponseWriter, r *http.Request) {
	data, _ := os.ReadFile("/home/cnf/projects/auto-coder-swarm/PROGRESS.md")
	h.render(w, "progress.html", map[string]interface{}{"Progress": string(data)})
}

func (h *DashboardHandler) HandleUpdateProgress(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	content := r.FormValue("content")
	os.WriteFile("/home/cnf/projects/auto-coder-swarm/PROGRESS.md", []byte(content), 0644)
	http.Redirect(w, r, "/progress", http.StatusSeeOther)
}

func (h *DashboardHandler) HandleLogs(w http.ResponseWriter, r *http.Request) {
	cmd := exec.Command("git", "-C", "/home/cnf/projects/auto-coder-swarm", "log", "-n", "20", "--oneline", "--decorate", "--graph")
	out, _ := cmd.CombinedOutput()
	h.render(w, "logs.html", map[string]interface{}{"Logs": string(out)})
}

func (h *DashboardHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /", h.HandleHome)
	mux.HandleFunc("GET /projects", h.HandleProjects)
	mux.HandleFunc("POST /projects/update", h.HandleUpdateProjects)
	mux.HandleFunc("GET /progress", h.HandleProgress)
	mux.HandleFunc("POST /progress/update", h.HandleUpdateProgress)
	mux.HandleFunc("GET /task", h.HandleTaskDetail)
	mux.HandleFunc("POST /task/stop", h.HandleStopTask)
	mux.HandleFunc("POST /task/approve", h.HandleApproveTask)
	mux.HandleFunc("POST /task/reject", h.HandleRejectTask)
	mux.HandleFunc("GET /logs", h.HandleLogs)
}
