package web

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
)

func (h *DashboardHandler) HandleSettings(w http.ResponseWriter, r *http.Request) {
	// **모델 목록은 vLLM 에서 가져온다.**
	//
	// 여기는 Ollama(:11434)를 불렀는데 그 서비스는 꺼져 있었고, 이제는 아예
	// 지웠다. 매 요청마다 조용히 실패해 목록이 늘 비어 있었다.
	// vLLM 은 OpenAI 호환이라 /v1/models 로 같은 것을 준다.
	base := os.Getenv("LLM_API_URL")
	if base == "" {
		base = "http://127.0.0.1:8000"
	}
	resp, err := http.Get(strings.TrimSuffix(base, "/") + "/v1/models")
	// vLLM 은 {"data":[{"id":"..."}]} 로 준다. Ollama 의 models[].name 과 모양이 다르다.
	var models struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
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
	// 화면은 이름만 쓴다. vLLM 은 파라미터 수를 주지 않으므로 이름으로 맞춘다.
	modelNames := make([]string, 0, len(models.Data))
	for _, m := range models.Data {
		modelNames = append(modelNames, m.ID)
	}

	apiKey := h.store.GetSetting("swarm_api_key")

	h.render(w, "settings.html", map[string]interface{}{
		"Models":       modelNames,
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
