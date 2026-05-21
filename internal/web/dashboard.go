package web

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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

// helpers defines custom functions for Go templates.
func (h *DashboardHandler) helpers() template.FuncMap {
	return template.FuncMap{
		"lower": strings.ToLower,
		"upper": strings.ToUpper,
		"json": func(v interface{}) string {
			b, _ := json.MarshalIndent(v, "", "  ")
			return string(b)
		},
		"formatTime": func(t time.Time) string {
			return t.Format("2006-01-02 15:04:05")
		},
	}
}

func (h *DashboardHandler) render(w http.ResponseWriter, page string, data interface{}) {
	// Root-cause Fix: In Go html/template, Funcs() MUST be called before ParseFiles().
	// Also, the template name used in New() must match the base name of one of the files being parsed.
	tmpl := template.New(filepath.Base(page)).Funcs(h.helpers())
	
	// Add layout and actual page to the template group
	layoutPath := filepath.Join(h.tmplPath, "layout.html")
	contentPath := filepath.Join(h.tmplPath, page)

	// Ensure layout is also named correctly if we want to use 'define "content"' patterns
	// A more robust way is to use template.ParseFiles with a shared FuncMap template
	baseTmpl := template.New("base").Funcs(h.helpers())
	parsedTmpl, err := baseTmpl.ParseFiles(layoutPath, contentPath)
	
	if err != nil {
		log.Printf("[Web] Template parse error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// We execute "layout.html" which contains the overall structure and calls {{template "content" .}}
	err = parsedTmpl.ExecuteTemplate(w, "layout.html", data)
	if err != nil {
		log.Printf("[Web] Template execution error: %v", err)
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

func (h *DashboardHandler) HandleSettings(w http.ResponseWriter, r *http.Request) {
	resp, err := http.Get("http://localhost:11434/api/tags")
	var models struct {
		Models []struct {
			Name    string `json:"name"`
			Details struct {
				ParameterSize string `json:"parameter_size"`
			} `json:"details"`
		} `json:"models"`
	}
	if err == nil {
		json.NewDecoder(resp.Body).Decode(&models)
		resp.Body.Close()
	}

	primary := h.store.GetSetting("primary_model")
	if primary == "" {
		primary = "gemma4:31b"
	}
	voterStr := h.store.GetSetting("voter_models")
	voterMap := make(map[string]bool)
	for _, m := range strings.Split(voterStr, ",") {
		if m != "" {
			voterMap[m] = true
		}
	}
	apiKey := h.store.GetSetting("swarm_api_key")

	data := map[string]interface{}{
		"Models":       models.Models,
		"PrimaryModel": primary,
		"VoterMap":     voterMap,
		"ApiKey":       apiKey,
	}
	h.render(w, "settings.html", data)
}

func (h *DashboardHandler) HandleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.ParseForm()
	primary := r.FormValue("primary_model")
	voters := r.Form["voter_models"]
	apiKey := r.FormValue("swarm_api_key")

	h.store.SaveSetting("primary_model", primary)
	h.store.SaveSetting("voter_models", strings.Join(voters, ","))
	h.store.SaveSetting("swarm_api_key", apiKey)

	os.Setenv("SWARM_API_KEY", apiKey)

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
