package web

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
)

func (h *DashboardHandler) HandleSettings(w http.ResponseWriter, r *http.Request) {
	resp, err := http.Get("http://localhost:11434/api/tags")
	var models struct {
		Models []struct {
			Name    string `json:"name"`
			Details struct {
				ParameterSize string `json:"parameter_size"`
			} `json:"details"`
		} `json:"models"`
	}
	if err == nil && resp != nil {
		json.NewDecoder(resp.Body).Decode(&models)
		resp.Body.Close()
	}

	primary := h.store.GetSetting("primary_model")
	log.Printf("[Dashboard] Loading settings. Primary model from DB: '%s'", primary)

	// [Step 52: Model Integrity Force Fix]
	// If the model is an embedding model or empty, force it back to gemma4
	if primary == "" || strings.Contains(primary, "bge-m3") {
		log.Printf("[Dashboard] WARNING: Inappropriate model detected ('%s'). Forcing gemma4:31b", primary)
		primary = "gemma4:31b"
		h.store.SaveSetting("primary_model", primary)
	}

	voters := h.store.GetSetting("voter_models")
	voterMap := make(map[string]bool)
	for _, m := range strings.Split(voters, ",") {
		if m != "" {
			voterMap[m] = true
		}
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
	newModel := r.FormValue("primary_model")

	// [Safety Guard] Prevent setting embedding models as primary
	if strings.Contains(newModel, "bge-m3") {
		log.Printf("[Dashboard] Blocked attempt to set embedding model as primary: %s", newModel)
		http.Redirect(w, r, "/settings?error=invalid_model", http.StatusSeeOther)
		return
	}

	h.store.SaveSetting("primary_model", newModel)
	h.store.SaveSetting("voter_models", strings.Join(r.Form["voter_models"], ","))
	h.store.SaveSetting("swarm_api_key", r.FormValue("swarm_api_key"))
	os.Setenv("SWARM_API_KEY", r.FormValue("swarm_api_key"))
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}
