package web

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/connectfit-team/auto-coder-swarm/internal/apikey"
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

	// **열쇠를 화면에 찍지 않는다.** 이 화면이 곧 열쇠 유출 경로가 된다.
	// 설정돼 있는지만 알린다.
	h.render(w, "settings.html", map[string]interface{}{
		"Models":       modelNames,
		"PrimaryModel": primary,
		"VoterMap":     voterMap,
		"KeySet":       apikey.Configured() != "",
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

	// 빈 칸은 "그대로 두라" 는 뜻이다. 화면에 열쇠를 찍지 않으니 빈 칸이 곧
	// 지우기가 되면, 모델만 바꾸려고 저장한 사람이 인증을 풀어 버린다.
	// 지우는 것은 따로 말해야 한다.
	switch k := strings.TrimSpace(r.FormValue("swarm_api_key")); {
	case k != "":
		h.store.SaveSetting("swarm_api_key", k)
		os.Setenv("SWARM_API_KEY", k)
		// 넣은 사람은 그 자리에서 계속 쓸 수 있어야 한다.
		apikey.SetCookie(w, k)
		log.Println("[Dashboard] SWARM_API_KEY 를 새로 넣었다")
	case r.FormValue("clear_api_key") != "":
		h.store.SaveSetting("swarm_api_key", "")
		os.Setenv("SWARM_API_KEY", "")
		log.Println("[Dashboard] SWARM_API_KEY 를 지웠다 — 인증이 풀렸다")
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}
