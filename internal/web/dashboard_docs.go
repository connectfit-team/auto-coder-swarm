package web

import (
	"net/http"
	"os"
	"os/exec"
)

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
