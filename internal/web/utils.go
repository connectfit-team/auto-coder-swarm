package web

import (
	"html/template"
	"net/http"
	"path/filepath"
)

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
