package web

import (
	"encoding/json"
	"html/template"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/storage"
	"github.com/connectfit-team/auto-coder-swarm/internal/stream"
	"github.com/connectfit-team/auto-coder-swarm/internal/worker"
)

type DashboardHandler struct {
	store    *storage.Storage
	worker   *worker.Manager
	stream   *stream.Manager
	tmplPath string
}

func NewDashboardHandler(s *storage.Storage, w *worker.Manager, sm *stream.Manager, tmplPath string) *DashboardHandler {
	return &DashboardHandler{store: s, worker: w, stream: sm, tmplPath: tmplPath}
}

type UISafeLog struct {
	CreatedAt  string
	Stage      string
	StageLower string
	Message    string
	Prompt     string
	Summary    string
}

func (h *DashboardHandler) render(w http.ResponseWriter, page string, data interface{}) {
	layoutPath := filepath.Join(h.tmplPath, "layout.html")
	contentPath := filepath.Join(h.tmplPath, page)
	tmpl, err := template.ParseFiles(layoutPath, contentPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tmpl.ExecuteTemplate(w, "layout.html", data)
}

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

func (h *DashboardHandler) HandleProjects(w http.ResponseWriter, r *http.Request) {
	data, _ := os.ReadFile("/home/cnf/projects/auto-coder-swarm/PROJECTS.md")
	h.render(w, "projects.html", map[string]interface{}{"Specs": string(data)})
}

func (h *DashboardHandler) HandleUpdateProjects(w http.ResponseWriter, r *http.Request) {
	content := r.FormValue("content")
	os.WriteFile("/home/cnf/projects/auto-coder-swarm/PROJECTS.md", []byte(content), 0644)
	http.Redirect(w, r, "/projects", http.StatusSeeOther)
}

func (h *DashboardHandler) HandleProgress(w http.ResponseWriter, r *http.Request) {
	data, _ := os.ReadFile("/home/cnf/projects/auto-coder-swarm/PROGRESS.md")
	h.render(w, "progress.html", map[string]interface{}{"Progress": string(data)})
}

func (h *DashboardHandler) HandleUpdateProgress(w http.ResponseWriter, r *http.Request) {
	content := r.FormValue("content")
	os.WriteFile("/home/cnf/projects/auto-coder-swarm/PROGRESS.md", []byte(content), 0644)
	http.Redirect(w, r, "/progress", http.StatusSeeOther)
}

func (h *DashboardHandler) HandleLogs(w http.ResponseWriter, r *http.Request) {
	gitOut, _ := exec.Command("git", "-C", "/home/cnf/projects/auto-coder-swarm", "log", "-n", "20", "--oneline", "--decorate", "--graph").CombinedOutput()
	svcOut, _ := exec.Command("tail", "-n", "100", "/home/cnf/projects/auto-coder-swarm/service.log").CombinedOutput()

	h.render(w, "logs.html", map[string]interface{}{
		"Logs":    string(gitOut),
		"SvcLogs": string(svcOut),
	})
}

func (h *DashboardHandler) HandleSettings(w http.ResponseWriter, r *http.Request) {
	resp, _ := http.Get("http://localhost:11434/api/tags")
	var models struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if resp != nil {
		json.NewDecoder(resp.Body).Decode(&models)
		resp.Body.Close()
	}

	primary := h.store.GetSetting("primary_model")
	voters := h.store.GetSetting("voter_models")
	voterMap := make(map[string]bool)
	for _, m := range strings.Split(voters, ",") {
		if m != "" { voterMap[m] = true }
	}
	apiKey := h.store.GetSetting("swarm_api_key")

	h.render(w, "settings.html", map[string]interface{}{
		"Models":       models.Models,
		"PrimaryModel": primary,
		"VoterMap":     voterMap,
		"ApiKey":       apiKey,
	})
}

func (h *DashboardHandler) HandleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	h.store.SaveSetting("primary_model", r.FormValue("primary_model"))
	h.store.SaveSetting("voter_models", strings.Join(r.Form["voter_models"], ","))
	h.store.SaveSetting("swarm_api_key", r.FormValue("swarm_api_key"))
	os.Setenv("SWARM_API_KEY", r.FormValue("swarm_api_key"))
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
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
	mux.HandleFunc("GET /task/stream", h.stream.ServeHTTP)
	mux.HandleFunc("GET /logs", h.HandleLogs)
	mux.HandleFunc("GET /settings", h.HandleSettings)
	mux.HandleFunc("POST /settings/update", h.HandleUpdateSettings)
}
