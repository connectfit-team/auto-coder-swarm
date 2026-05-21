package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/connectfit-team/auto-coder-swarm/internal/storage"
	"github.com/connectfit-team/auto-coder-swarm/internal/stream"
	"github.com/connectfit-team/auto-coder-swarm/internal/worker"
)

func TestDashboardHandler_HandleHome(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "templates")
	defer os.RemoveAll(tmpDir)

	os.WriteFile(filepath.Join(tmpDir, "layout.html"), []byte("{{define \"layout.html\"}}{{template \"content\" .}}{{end}}"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "home.html"), []byte("{{define \"content\"}}Swarm Home{{end}}"), 0644)

	dbPath := filepath.Join(tmpDir, "test.db")
	store, _ := storage.NewStorage(dbPath)
	wm := worker.NewManager()
	sm := stream.NewManager(store)

	handler := NewDashboardHandler(store, wm, sm, tmpDir)

	req, _ := http.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	handler.HandleHome(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	if rr.Body.String() != "Swarm Home" {
		t.Errorf("handler returned unexpected body: %s", rr.Body.String())
	}
}
