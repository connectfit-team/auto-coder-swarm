package api

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/connectfit-team/auto-coder-swarm/internal/insightclient"
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

func (h *SwarmHandler) HandleGenerateReport(w http.ResponseWriter, r *http.Request) {
	report, err := h.reporter.GenerateDailyReport(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/markdown")
	fmt.Fprint(w, report)
}

func (h *SwarmHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/tasks", h.requestLogger(h.enableCORS(h.HandleListTasks)))
	mux.HandleFunc("GET /api/v1/tasks/detail", h.requestLogger(h.enableCORS(h.HandleGetTask)))
	mux.HandleFunc("POST /api/v1/tasks", h.requestLogger(h.enableCORS(h.HandleSubmitTask)))
	mux.HandleFunc("POST /api/v1/tasks/stop", h.requestLogger(h.enableCORS(h.HandleStopTask)))
	mux.HandleFunc("GET /api/v1/settings", h.requestLogger(h.enableCORS(h.HandleGetSettings)))
	mux.HandleFunc("POST /api/v1/settings", h.requestLogger(h.enableCORS(h.HandleUpdateSettings)))
	mux.HandleFunc("GET /api/v1/report/daily", h.requestLogger(h.enableCORS(h.HandleGenerateReport)))
	mux.HandleFunc("POST /api/v1/chat", h.requestLogger(h.enableCORS(h.HandleChatSubmission)))

	mux.HandleFunc("POST /api/v1/approve", h.requestLogger(h.enableCORS(func(w http.ResponseWriter, r *http.Request) {
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
	})))
}
