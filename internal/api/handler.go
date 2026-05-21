package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/connectfit-team/auto-coder-swarm/internal/insightclient"
	"github.com/connectfit-team/auto-coder-swarm/internal/orchestrator"
	"github.com/connectfit-team/auto-coder-swarm/internal/reporting"
	"github.com/connectfit-team/auto-coder-swarm/internal/storage"
	"github.com/connectfit-team/auto-coder-swarm/internal/worker"
)

type SwarmHandler struct {
	store    *storage.Storage
	worker   *worker.Manager
	reporter *reporting.Service
	insight  *insightclient.Client
}

func NewSwarmHandler(s *storage.Storage, w *worker.Manager, rs *reporting.Service, ic *insightclient.Client) *SwarmHandler {
	return &SwarmHandler{store: s, worker: w, reporter: rs, insight: ic}
}

// Middleware: Request Logger
func (h *SwarmHandler) requestLogger(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("[API] %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
		next(w, r)
		log.Printf("[API] Completed %s %s in %v", r.Method, r.URL.Path, time.Since(start))
	}
}

// Middleware: CORS
func (h *SwarmHandler) enableCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next(w, r)
	}
}

func (h *SwarmHandler) checkAuth(r *http.Request) bool {
	expectedKey := os.Getenv("SWARM_API_KEY")
	if expectedKey == "" {
		return true
	}
	clientKey := r.Header.Get("X-API-Key")
	return clientKey == expectedKey
}

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

func (h *SwarmHandler) HandleGenerateReport(w http.ResponseWriter, r *http.Request) {
	report, err := h.reporter.GenerateDailyReport(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/markdown")
	fmt.Fprint(w, report)
}

func (h *SwarmHandler) HandleGetSettings(w http.ResponseWriter, r *http.Request) {
	resp, err := http.Get("http://localhost:11434/api/tags")
	var ollamaResp struct {
		Models []interface{} `json:"models"`
	}
	if err == nil {
		json.NewDecoder(resp.Body).Decode(&ollamaResp)
		resp.Body.Close()
	}

	primary := h.store.GetSetting("primary_model")
	voters := h.store.GetSetting("voter_models")
	apiKey := h.store.GetSetting("swarm_api_key")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"available_models": ollamaResp.Models,
		"primary_model":    primary,
		"voter_models":      strings.Split(voters, ","),
		"api_key_set":      apiKey != "",
	})
}

func (h *SwarmHandler) HandleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	if !h.checkAuth(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	var settings map[string]interface{}
	json.NewDecoder(r.Body).Decode(&settings)

	if v, ok := settings["primary_model"].(string); ok {
		h.store.SaveSetting("primary_model", v)
	}
	if v, ok := settings["voter_models"].([]interface{}); ok {
		var names []string
		for _, m := range v { names = append(names, m.(string)) }
		h.store.SaveSetting("voter_models", strings.Join(names, ","))
	}
	if v, ok := settings["swarm_api_key"].(string); ok {
		h.store.SaveSetting("swarm_api_key", v)
		os.Setenv("SWARM_API_KEY", v)
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Settings updated")
}

func (h *SwarmHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/tasks", h.requestLogger(h.enableCORS(h.HandleListTasks)))
	mux.HandleFunc("GET /api/v1/tasks/detail", h.requestLogger(h.enableCORS(h.HandleGetTask)))
	mux.HandleFunc("POST /api/v1/tasks", h.requestLogger(h.enableCORS(h.HandleSubmitTask)))
	mux.HandleFunc("POST /api/v1/tasks/stop", h.requestLogger(h.enableCORS(h.HandleStopTask)))
	mux.HandleFunc("GET /api/v1/settings", h.requestLogger(h.enableCORS(h.HandleGetSettings)))
	mux.HandleFunc("POST /api/v1/settings", h.requestLogger(h.enableCORS(h.HandleUpdateSettings)))
	mux.HandleFunc("GET /api/v1/report/daily", h.requestLogger(h.enableCORS(h.HandleGenerateReport)))
	
	mux.HandleFunc("POST /api/v1/approve", h.requestLogger(h.enableCORS(func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		h.store.UpdateTaskStatus(id, storage.StatusApproved, "", "")
		fmt.Fprintf(w, "Task %s approved", id)
	})))
}
