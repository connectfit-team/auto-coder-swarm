package web

import (
	"net/http"

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

// routes 는 대시보드가 여는 길 전부다. 열쇠 문은 등록하는 자리에서 채운다 —
// 화면 하나라도 문 밖에 두면 그 화면이 곧 구멍이 된다. /settings 는 설정을
// 바꾸고, /task/approve 는 PR 을 연다.
func (h *DashboardHandler) routes() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"GET /":                 h.HandleHome,
		"GET /projects":         h.HandleProjects,
		"POST /projects/update": h.HandleUpdateProjects,
		"GET /progress":         h.HandleProgress,
		"POST /progress/update": h.HandleUpdateProgress,
		"GET /task":             h.HandleTaskDetail,
		"POST /task/stop":       h.HandleStopTask,
		"POST /task/approve":    h.HandleApproveTask,
		"POST /task/reject":     h.HandleRejectTask,
		"GET /task/stream":      h.stream.ServeHTTP,
		"GET /logs":             h.HandleLogs,
		"GET /settings":         h.HandleSettings,
		"POST /settings/update": h.HandleUpdateSettings,
		"GET /chat":             h.HandleChatView,
	}
}

func (h *DashboardHandler) RegisterRoutes(mux *http.ServeMux) {
	// 문 밖에 두는 것은 열쇠를 받는 자리뿐이다.
	mux.HandleFunc("GET /unlock", h.HandleUnlock)
	mux.HandleFunc("POST /unlock", h.HandleUnlockSubmit)

	for pattern, fn := range h.routes() {
		mux.HandleFunc(pattern, h.gate(fn))
	}
}
