package web

import (
	"html/template"
	"net/http"
	"path/filepath"

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
	mux.HandleFunc("GET /chat", h.HandleChatView)
}
