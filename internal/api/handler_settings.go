package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

func (h *SwarmHandler) HandleGetSettings(w http.ResponseWriter, r *http.Request) {
	req, err := http.NewRequest("GET", "http://127.0.0.1:8000/v1/models", nil)
	if err == nil {
		req.Header.Set("Authorization", "Bearer gemma4-secret-key-9988")
	}
	resp, err := http.DefaultClient.Do(req)

	var models []map[string]interface{}
	var vllmResp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err == nil {
		json.NewDecoder(resp.Body).Decode(&vllmResp)
		resp.Body.Close()
		for _, m := range vllmResp.Data {
			models = append(models, map[string]interface{}{"name": m.ID})
		}
	}

	primary := h.store.GetSetting("primary_model")
	voters := h.store.GetSetting("voter_models")
	apiKey := h.store.GetSetting("swarm_api_key")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"available_models": models,
		"primary_model":    primary,
		"voter_models":     strings.Split(voters, ","),
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
		for _, m := range v {
			names = append(names, m.(string))
		}
		h.store.SaveSetting("voter_models", strings.Join(names, ","))
	}
	if v, ok := settings["swarm_api_key"].(string); ok {
		h.store.SaveSetting("swarm_api_key", v)
		os.Setenv("SWARM_API_KEY", v)
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Settings updated")
}
