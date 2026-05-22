package web

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
)

func (h *DashboardHandler) HandleSettings(w http.ResponseWriter, r *http.Request) {
	resp, _ := http.Get("http://localhost:11434/api/tags")
	var models struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if resp != nil {
		json.NewDecoder(resp.Body).Decode(&models)
		resp.Body.Close()
	}

	primary := h.store.GetSetting("primary_model")
	voters := h.store.GetSetting("voter_models")
	voterMap := make(map[string]bool)
	for _, m := range strings.Split(voters, ",") {
		if m != "" { voterMap[m] = true }
	}
	apiKey := h.store.GetSetting("swarm_api_key")

	h.render(w, "settings.html", map[string]interface{}{
		"Models":       models.Models,
		"PrimaryModel": primary,
		"VoterMap":     voterMap,
		"ApiKey":       apiKey,
	})
}

func (h *DashboardHandler) HandleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	h.store.SaveSetting("primary_model", r.FormValue("primary_model"))
	h.store.SaveSetting("voter_models", strings.Join(r.Form["voter_models"], ","))
	h.store.SaveSetting("swarm_api_key", r.FormValue("swarm_api_key"))
	os.Setenv("SWARM_API_KEY", r.FormValue("swarm_api_key"))
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}
