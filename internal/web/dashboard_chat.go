package web

import (
	"net/http"
)

func (h *DashboardHandler) HandleChatView(w http.ResponseWriter, r *http.Request) {
	// A new interactive chat page where the user can submit a task and see results immediately
	h.render(w, "chat.html", nil)
}
