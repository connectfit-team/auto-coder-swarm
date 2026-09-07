package api

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/connectfit-team/auto-coder-swarm/internal/apikey"
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
//
// 예비 요청(OPTIONS)은 여기로 오지 않는다 — 길을 "GET /..." 처럼 방법까지
// 묶어 등록하므로 mux 가 먼저 405 를 준다. 그래서 예비 요청은 따로 등록한다.
func (h *SwarmHandler) enableCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		next(w, r)
	}
}

func setCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key")
}

// handlePreflight 는 브라우저의 예비 요청에 답한다.
// 예비 요청에는 열쇠가 없다 — 열쇠를 물으면 뒤따르는 본 요청이 아예 오지 않는다.
func (h *SwarmHandler) handlePreflight(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	w.WriteHeader(http.StatusOK)
}

// requireKey 는 열쇠를 확인한다.
//
// 손잡이마다 checkAuth 를 부르는 방식이었는데 아홉 중 다섯이 빠져 있었다.
// 그 가운데 POST /api/v1/approve 는 사람의 승인을 대신해 고친 것을 밀어
// 올리고 PR 을 연다. 등록하는 자리에서 한 번 감싸면 빠뜨릴 수 없다.
func (h *SwarmHandler) requireKey(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !apikey.Allowed(r) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// wrap 은 모든 손잡이가 지나는 길이다: 기록 → CORS → 열쇠.
// CORS 가 열쇠보다 앞이라야 한다 — 브라우저의 예비 요청(OPTIONS)에는
// 열쇠가 없다.
func (h *SwarmHandler) wrap(next http.HandlerFunc) http.HandlerFunc {
	return h.requestLogger(h.enableCORS(h.requireKey(next)))
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

// routes 는 이 서비스가 여는 길 전부다. 등록도 시험도 이 목록을 본다 —
// 목록 밖에 길이 없으면 열쇠를 빠뜨릴 수도 없다.
func (h *SwarmHandler) routes() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"GET /api/v1/tasks":        h.HandleListTasks,
		"GET /api/v1/tasks/detail": h.HandleGetTask,
		"POST /api/v1/tasks":       h.HandleSubmitTask,
		"POST /api/v1/tasks/stop":  h.HandleStopTask,
		"GET /api/v1/settings":     h.HandleGetSettings,
		"POST /api/v1/settings":    h.HandleUpdateSettings,
		"GET /api/v1/report/daily": h.HandleGenerateReport,
		"POST /api/v1/chat":        h.HandleChatSubmission,
		"POST /api/v1/approve":     h.HandleApproveTask,
	}
}

func (h *SwarmHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("OPTIONS /api/v1/", h.handlePreflight)

	for pattern, fn := range h.routes() {
		mux.HandleFunc(pattern, h.wrap(fn))
	}
}
